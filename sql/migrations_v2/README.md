# migrations_v2 — clean schema rebuild

Plain SQL, no migration tool required. One numbered pair per table; apply
the `.up.sql` files in numeric order (down files in reverse order):

```sh
for f in $(ls *.up.sql | sort); do
  psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -f "$f"
done
```

File names stay golang-migrate compatible (`NNN_name.up/down.sql`), so the
tool can be adopted later without renaming.

## Design rules

- **PKs:** `uuid` via `uuid_generate_v7()` (time-ordered, defined in `000_setup`).
- **No Postgres enums.** Closed vocabularies are `text` + `CHECK` — changing
  them later is a one-line constraint swap, and Go/sqlc sees plain strings.
- **No polymorphic tables.** Saved items are three concrete tables
  (`saved_articles`, `saved_goals`, `saved_habits`) with real FKs.
  `plan_adjustments` targets goal XOR habit via two nullable FKs + a CHECK.
- **One global `categories` table.** `articles`, `goals`, and `habits` all
  reference it with `category_id ... ON DELETE SET NULL`.
- **Minimal indexes.** Roughly one per real query pattern, usually
  `(user_id, created_at DESC)`. Add more only when a slow query proves the need.
- **Triggers kept to two jobs only:** `set_updated_at` and the search outbox
  enqueue. Everything else (immutability rules, derived flags, counters) is
  application logic.
- **No RLS** — authorization is enforced in the services.

## What changed vs the old schema

| Old | New |
|---|---|
| `profiles` table | merged into `users` (bio, location, website, avatar_url) |
| `user_coaching_profiles` | merged into `user_settings` |
| `saved_items` + validation/cleanup triggers | deleted; concrete tables only |
| `categories.entity_type` + free-text `goals.category` / `habits.category` | one global `categories`, FK from all three entities |
| `goals.completed` bool + sync trigger | derive from `status = 'completed'` |
| `habits.completed` | derive from today's check-in |
| `version` columns + `(id, version)` indexes | dropped (last write wins) |
| 20 enum types | `text` + `CHECK` |
| `articles.search_vector` trigger | `GENERATED ALWAYS ... STORED` column |
| `weekly_reviews.week_end` | derive as `week_start + 6` |
| `reminder_queue.sent` bool + `sent_at` | `sent_at IS NULL` = pending; table renamed `reminders` |
| `processed_events` + `ai_coach_processed_events` + `processed_stripe_events` | one `processed_events (consumer, event_id)` |
| outbox with status/locking/coalescing upsert | insert-only `search_outbox`; worker deletes processed rows |
| trigram / jsonb GIN / duplicate & low-selectivity indexes | removed |
| `upgrade_events.trigger` | renamed `trigger_source` |

## Re-adding things on purpose, not by default

- Optimistic locking: add `version integer NOT NULL DEFAULT 1` back to a
  table only if a real concurrent-edit conflict shows up (the PK already
  serves the `WHERE id = $1 AND version = $2` update).
- New index: justify with an actual query, and create it `CONCURRENTLY`
  once tables hold production data.

## Follow-ups in the backend

- Regenerate sqlc models/queries against this schema (table/column renames:
  `goal_habit_relations` → `goal_habits`, `user_subscriptions` → `subscriptions`,
  `reminder_queue` → `reminders`, `activities.item_type` → `type`,
  `notifications.item_type` → `type`, `read_time` → `read_time_minutes`).
- Update the search-sync worker to the new outbox contract (read by `id`,
  delete on success) and the per-consumer `processed_events` key.
- Drop the legacy `SavedItemsRepo` methods that touched `saved_items`.
