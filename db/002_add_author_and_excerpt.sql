-- Adds author_name and excerpt to digitalgarden_note, both denormalized
-- (no join against digitalgarden_user): author_name is a snapshot of the
-- author's display name taken at publish time, same pattern as
-- onlineshopping-backend's Order.CommodityName — so renaming a user later
-- doesn't rewrite the byline on notes they already published, and the
-- public feed (reads >> writes) never pays a join to render a byline.
-- excerpt is a short preview computed at publish time (first paragraph's
-- text, truncated), same "compute once, serve as-is" reasoning as
-- parsed_tree.
--
-- Run this against the already-seeded database from db/schema.sql — it's
-- an incremental change, not a from-scratch script. db/schema.sql itself
-- has also been updated to include these columns from the start, for
-- anyone setting up fresh.

alter table digitalgarden.digitalgarden_note
  add column if not exists author_name text,
  add column if not exists excerpt text;

update digitalgarden.digitalgarden_note
   set author_name = 'Cortalo',
       excerpt = 'This is a paragraph.'
 where slug = 'hello-world';

update digitalgarden.digitalgarden_note
   set author_name = 'Cortalo',
       excerpt = 'This is a paragraph.'
 where slug = 'a-plain-note';

-- Enforced only after backfilling existing rows above — adding NOT NULL
-- directly in the same statement as ADD COLUMN would fail against a
-- table that already has rows.
alter table digitalgarden.digitalgarden_note
  alter column author_name set not null,
  alter column excerpt set not null;
