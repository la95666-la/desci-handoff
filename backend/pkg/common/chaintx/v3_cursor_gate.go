package chaintx

import (
	"encoding/hex"
	"errors"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"gorm.io/gorm"

	"joincare_server/pkg/common/idov3whitelist"
)

// LockV3RunningCursorTx takes the scanner's cursor lock before the project
// row. Every Core lifecycle writer follows cursor -> project -> submission ->
// sender, matching the scanner's order; a halted scanner cannot sign or send.
func LockV3RunningCursorTx(tx *gorm.DB, binding idov3whitelist.ExpectedProjectBinding) error {
	invalid := errors.New("V3 canonical cursor unavailable")
	if tx == nil || binding.ChainID == 0 || binding.ProjectID == 0 || binding.ProxyAddress == (common.Address{}) {
		return invalid
	}
	var rows []struct {
		Block    uint64 `gorm:"column:cursor_block"`
		Hash     string `gorm:"column:cursor_hash"`
		Revision uint64 `gorm:"column:revision"`
		State    string `gorm:"column:state"`
	}
	read := tx.Raw(`SELECT cursor_block,cursor_hash,revision,state FROM ido_v3_scan_cursor
		WHERE chain_id=? AND proxy_address=? AND project_id=? AND contract_version='ido_v3_stablecoin'
		LIMIT 2 FOR UPDATE`, binding.ChainID, strings.ToLower(binding.ProxyAddress.Hex()), binding.ProjectID).Scan(&rows)
	if read.Error != nil || len(rows) != 1 {
		return invalid
	}
	digest := rows[0].Hash
	_, hashErr := hex.DecodeString(strings.TrimPrefix(digest, "0x"))
	if rows[0].State != "running" || rows[0].Block == 0 ||
		rows[0].Revision == 0 || len(digest) != 66 || !strings.HasPrefix(digest, "0x") ||
		digest != strings.ToLower(digest) || hashErr != nil || common.HexToHash(digest) == (common.Hash{}) {
		return invalid
	}
	var checkpoint []struct {
		Hash string `gorm:"column:block_hash"`
	}
	// The incident gate may already have established a REPEATABLE READ
	// snapshot. FOR UPDATE above reads the CURRENT cursor, not that snapshot.
	// Its checkpoint must also use a current locking read, otherwise a normal
	// Job advance committed since the snapshot falsely looks like a missing
	// checkpoint. Keep cursor -> checkpoint order; share-lock the exact fact
	// until commit so it cannot be orphaned while this gate authorizes a write.
	read = tx.Raw(`SELECT block_hash FROM ido_v3_block_checkpoint WHERE chain_id=? AND proxy_address=? AND project_id=?
		AND contract_version='ido_v3_stablecoin' AND block_number=? AND block_hash=? AND status='canonical' LIMIT 2 FOR SHARE`,
		binding.ChainID, strings.ToLower(binding.ProxyAddress.Hex()), binding.ProjectID, rows[0].Block, rows[0].Hash).Scan(&checkpoint)
	if read.Error != nil || len(checkpoint) != 1 || checkpoint[0].Hash != rows[0].Hash {
		return invalid
	}
	return nil
}
