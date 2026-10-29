# Publication (Prompt №13)

Stages 15–19:

15. `BUILD_PUBLICATION` — immutable `PUBLICATION_CANDIDATE` only (no `INSERT` into `analysis_publications`)
16. `BUILD_PROJECTIONS` — typed artifacts; sealed Factual API allowed; no Ozon/live
17. `VALIDATE_PUBLICATION` — deterministic gate
18. `PUBLISH` — one PostgreSQL transaction: seller `pg_advisory_xact_lock`, verify current pointer == run `base_publication_id`, insert publication + findings/recs + apply Action plan + projections + current pointer + release events + outbox; finalize PUBLISH + ready POST_PUBLISH **in the same TX**. Durable success is DB commit. No MinIO inside the TX.
19. `POST_PUBLISH` — outbox relay (at-least-once). Failure cannot unpublish.

`EVALUATION` execution class cannot insert publications or mutate current pointer.

Stale loser (current pointer moved): `STALE_PUBLISH`, zero publication writes.

Current pointer: `seller_current_publications` only (never `ORDER BY published_at`).

Immutability: UPDATE/DELETE triggers on publication-owned content.

See [publication-projections.md](publication-projections.md).
