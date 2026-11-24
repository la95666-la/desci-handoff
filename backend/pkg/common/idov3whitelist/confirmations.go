package idov3whitelist

// HasConfirmations counts the observed block itself as one. Subtract before
// comparing so a provider reporting the maximum uint64 head cannot wrap the
// depth to zero. This is a point-in-time depth check, not finality or proof of
// a canonical block; callers must independently verify the block hash.
func HasConfirmations(head, block, minimum uint64) bool {
	return block > 0 && minimum > 0 && head >= block && head-block >= minimum-1
}
