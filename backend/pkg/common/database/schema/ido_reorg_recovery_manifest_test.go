package schema

import "testing"

func TestApprovedChainEventReorgRecoveryDefinitionsAreFrozen(t *testing.T) {
	if len(approvedChainEventReorgRecoveryDefinitions) != 31 {
		t.Fatalf("unexpected approved definition count: %d", len(approvedChainEventReorgRecoveryDefinitions))
	}
	for key, digest := range approvedChainEventReorgRecoveryDefinitions {
		if len(digest) != 64 || key == "" {
			t.Fatalf("invalid approved definition %q=%q", key, digest)
		}
	}
}

func TestCanonicalSQLTokensOnlyIgnoresFormatting(t *testing.T) {
	left := canonicalSQLTokens("BEGIN IF NEW.event_type = 'CREATED' THEN SIGNAL SQLSTATE '45000'; END IF; END")
	right := canonicalSQLTokens("begin\n if `NEW`.`event_type`='CREATED' then signal sqlstate '45000';end if;end")
	if left != right {
		t.Fatalf("format-only SQL difference changed canonical tokens:\n%s\n%s", left, right)
	}
	if canonicalSQLTokens("state='scanning'") == canonicalSQLTokens("state='completed'") {
		t.Fatal("string literal semantic change was normalized away")
	}
	if canonicalSQLTokens("next_block=target+1") == canonicalSQLTokens("next_block>=target+1") {
		t.Fatal("operator semantic change was normalized away")
	}
}

func TestChainEventReorgRecoveryDefinitionDigestBindsAttachment(t *testing.T) {
	base := chainEventReorgRecoveryDefinition{
		ObjectType: "TRIGGER", ObjectName: "example", ObjectTable: "chain_event_reorg_recovery_log",
		ActionTiming: "BEFORE", EventManipulation: "DELETE", ActionOrientation: "ROW",
		Definition: "BEGIN SIGNAL SQLSTATE '45000'; END",
	}
	first, err := chainEventReorgRecoveryDefinitionDigest(base)
	if err != nil {
		t.Fatal(err)
	}
	base.ObjectTable = "chain_event_reorg_recovery_batch"
	second, err := chainEventReorgRecoveryDefinitionDigest(base)
	if err != nil {
		t.Fatal(err)
	}
	if first == second {
		t.Fatal("trigger attachment change did not change approved digest")
	}
	base.ObjectTable = "Chain_Event_Reorg_Recovery_Log"
	third, err := chainEventReorgRecoveryDefinitionDigest(base)
	if err != nil {
		t.Fatal(err)
	}
	if first == third {
		t.Fatal("case-sensitive trigger attachment change did not change approved digest")
	}
}

func TestChainEventReorgRecoveryDefinitionRejectsCaseChangedTableReference(t *testing.T) {
	definition := chainEventReorgRecoveryDefinition{
		ObjectType: "TRIGGER", ObjectTable: "chain_event_reorg_recovery_log", ActionTiming: "BEFORE",
		EventManipulation: "INSERT", ActionOrientation: "ROW",
		Definition: "BEGIN SELECT id FROM Chain_Event_Reorg_Incident; END",
	}
	if _, err := chainEventReorgRecoveryDefinitionDigest(definition); err == nil {
		t.Fatal("case-changed chain-event table reference was accepted")
	}
}
