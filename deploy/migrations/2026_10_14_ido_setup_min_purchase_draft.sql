-- Setup-stage intent in display units; never an effective purchase policy.
-- Set converts this intent to min_purchase_amount_raw under trusted token
-- decimals and the on-chain max-per-user, then clears both draft columns.
ALTER TABLE projects
  ADD COLUMN min_purchase_draft_amount VARCHAR(78) CHARACTER SET ascii COLLATE ascii_bin NULL AFTER min_purchase_amount_raw,
  ADD COLUMN min_purchase_draft_token VARCHAR(8) CHARACTER SET ascii COLLATE ascii_bin NULL AFTER min_purchase_draft_amount;
