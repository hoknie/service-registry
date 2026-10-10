# Documentation search: configuring `KNOWLEDGE_SEARCH_ENGINE`

Project documentation is searched through the search page (`/<locale>/search`),
`GET /api/v1/knowledge/search` and the MCP tool `search_docs`. One engine serves all three; it is
chosen by `KNOWLEDGE_SEARCH_ENGINE`. The user picks a mode for every query:

- `text` — by words;
- `semantic` — by meaning, through embeddings;
- `hybrid` — mixed.

## 1. Choosing an engine

| Engine | Modes | Needs | Choose it when |
|---|---|---|---|
| `postgres` (default) | `text` | nothing | word search is enough |
| `pgvector` | `text`, `semantic`, `hybrid` | the `vector` extension in Postgres + an embeddings API | you want search by meaning without another service |
| `qdrant` | `text`, `semantic`, `hybrid` | Qdrant + an embeddings API | you already run Qdrant or have a lot of documentation |
| `meilisearch` | `text`; with embeddings also `semantic`, `hybrid` | Meilisearch; embeddings are optional | you want typo tolerance and word-prefix search |

How the modes work:

- **`text` on `pgvector` and `qdrant`** is the same Postgres full-text search as `postgres`:
  Russian and English word stems, word beginnings, phrases, `or`, `-exclusion`.
  On `meilisearch`, `text` is Meilisearch's own search, which tolerates typos.
- **`hybrid` on `pgvector` and `qdrant`** merges the `text` and `semantic` results by rank
  (Reciprocal Rank Fusion). On `meilisearch` it is Meilisearch hybrid search with weight 0.5.
- **The default mode** is `hybrid` when embeddings are configured, `text` otherwise.

**Recommendation.** Start with `postgres`. If you need search by meaning, take `pgvector`: it
lives in the same database and needs no extra service. `qdrant` and `meilisearch` make sense when
you already run that service or need its specific features.

One `serve` process uses one engine. To compare engines, change the variable and restart `serve`.

## 2. Embeddings (for `semantic` and `hybrid`)

Any OpenAI-compatible API works: OpenAI, Ollama, vLLM, LM Studio, LocalAI. The registry sends
documentation fragments to `POST {EMBEDDINGS_URL}/embeddings`.

| Variable | Required | Default | Meaning |
|---|---|---|---|
| `EMBEDDINGS_URL` | for `pgvector`, `qdrant`; optional for `meilisearch` | — | API base URL, usually ending in `/v1` |
| `EMBEDDINGS_MODEL` | with `EMBEDDINGS_URL` | — | model name |
| `EMBEDDINGS_DIMENSIONS` | with `EMBEDDINGS_URL` | — | vector size of the model (8…8192) |
| `EMBEDDINGS_API_KEY` | no | — | API key: a value or a reference `env:NAME` / `file:/path` |
| `EMBEDDINGS_BATCH` | no | `32` | fragments per request (1…512) |
| `KNOWLEDGE_CHUNK_CHARS` | no | `1500` | maximum fragment length in characters (200…8000) |
| `KNOWLEDGE_INDEX_INTERVAL_SECS` | no | `60` | how often a project is rechecked (5…86400) |

`EMBEDDINGS_DIMENSIONS` must match the model's native vector size. The registry does not ask the
API to shorten vectors and rejects a response of another length: the file stays queued and a
warning appears in the log.

| Model | Where | `EMBEDDINGS_DIMENSIONS` |
|---|---|---|
| `text-embedding-3-small` | OpenAI | `1536` |
| `text-embedding-3-large` | OpenAI | `3072` |
| `nomic-embed-text` | Ollama | `768` |
| `mxbai-embed-large` | Ollama | `1024` |
| `bge-m3` | Ollama / vLLM (handles Russian well) | `1024` |

**OpenAI:**
```sh
EMBEDDINGS_URL=https://api.openai.com/v1
EMBEDDINGS_MODEL=text-embedding-3-small
EMBEDDINGS_DIMENSIONS=1536
EMBEDDINGS_API_KEY=env:OPENAI_API_KEY
```

**Local Ollama** — documentation never leaves your network:
```sh
docker run -d --name ollama -p 11434:11434 -v ollama:/root/.ollama ollama/ollama
docker exec ollama ollama pull bge-m3
```
```sh
EMBEDDINGS_URL=http://localhost:11434/v1
EMBEDDINGS_MODEL=bge-m3
EMBEDDINGS_DIMENSIONS=1024
```

With a cloud API, documentation text is sent to the provider. If that is not acceptable, use a
local model.

## 3. Setting up each engine

### `postgres`

```sh
KNOWLEDGE_SEARCH_ENGINE=postgres
```

Nothing else is needed. There is no background indexing: search runs over the collected snapshots.

### `pgvector`

```sh
KNOWLEDGE_SEARCH_ENGINE=pgvector
EMBEDDINGS_URL=http://localhost:11434/v1
EMBEDDINGS_MODEL=bge-m3
EMBEDDINGS_DIMENSIONS=1024
```

The `vector` extension must be available in the same database:

- **Self-hosted Postgres.** Use an image with pgvector (`pgvector/pgvector:0.8.6-pg17`) or the
  `postgresql-17-pgvector` package.
- **Managed Postgres** (RDS, Cloud SQL, Yandex MDB, …). Enable the extension in the cluster
  settings.
- **New database.** The migration creates the extension itself when the server has it.
- **Extension installed after the migrations ran.** Create it by hand:
  ```sql
  CREATE EXTENSION IF NOT EXISTS vector SCHEMA public;
  ```
- **No extension.** `serve` refuses to start with
  `KNOWLEDGE_SEARCH_ENGINE=pgvector needs the PostgreSQL extension "vector" …`.

The dev database (`pgsql` in compose) is `postgres:17-alpine` by default, without pgvector. To
switch it:

```sh
# .env
PGSQL_IMAGE=pgvector/pgvector:0.8.6-pg17
```

The image uses a different libc, so string sorting changes. On an existing volume created by the
alpine image, start from a clean database:

```sh
docker compose rm -sf pgsql
docker volume rm svc-registry_pgsql-data   # deletes the dev database data
just dev                                   # starts pgsql and applies the migrations
```

If you need to keep the data, keep the volume and run the `CREATE EXTENSION` above.

Vectors are stored in `knowledge_embeddings`. Search is exact, without an HNSW index; that is
enough for thousands to tens of thousands of fragments.

### `qdrant`

```sh
KNOWLEDGE_SEARCH_ENGINE=qdrant
QDRANT_URL=localhost:6334
QDRANT_COLLECTION=svc_registry_docs
EMBEDDINGS_URL=http://localhost:11434/v1
EMBEDDINGS_MODEL=bge-m3
EMBEDDINGS_DIMENSIONS=1024
```

- **`QDRANT_URL` is the `host:port` of the gRPC API (6334), not the HTTP port 6333.** Do not add
  `http://`.
- **The collection** is created on first use with size `EMBEDDINGS_DIMENSIONS` and cosine
  distance.
- **When you switch to a model with another vector size,** set a new `QDRANT_COLLECTION` or
  recreate the collection: the old one was created for the old size. The registry compares the
  size of an existing collection with `EMBEDDINGS_DIMENSIONS` at startup and before the first write
  or search. On a mismatch it never touches the collection: `serve` logs a warning with
  `code=search.engine_dimensions`, indexing fails with that code and the `semantic`/`hybrid` modes
  answer `503`.
- **`QDRANT_API_KEY`** is needed only when Qdrant has a key (`QDRANT__SERVICE__API_KEY`). A value
  or an `env:` / `file:` reference.
- **The registry's Qdrant client does not use TLS yet.** Keep Qdrant on the internal network next
  to the registry.

For development:

```sh
just docker-vdb up qdrant   # gRPC :6334, dashboard http://localhost:6333/dashboard
```

### `meilisearch`

Without embeddings only `text` is available, with typo tolerance:

```sh
KNOWLEDGE_SEARCH_ENGINE=meilisearch
MEILISEARCH_URL=http://localhost:7700
MEILISEARCH_INDEX=svc_registry_docs
```

With embeddings, `semantic` and `hybrid` are added:

```sh
EMBEDDINGS_URL=http://localhost:11434/v1
EMBEDDINGS_MODEL=bge-m3
EMBEDDINGS_DIMENSIONS=1024
```

- **The registry computes the vectors** and passes them to Meilisearch through the embedder
  `default` with `source: userProvided`. Meilisearch never calls the embeddings API itself.
- **The index** is created on first use. The registry sets the filterable and searchable
  attributes, `distinctAttribute` and the embedder.
- **`MEILISEARCH_API_KEY`** is needed only when the server has `MEILI_MASTER_KEY`. The key needs
  rights to create indexes, update settings, manage documents, search and read tasks; the simplest
  choice is the `Default Admin API Key`. A value or an `env:` / `file:` reference.

For development:

```sh
just docker-vdb up meilisearch   # :7700, no key (MEILI_ENV=development)
```

## 4. How and when documents reach the index

Indexing is a background job inside `serve`. It runs when all of these hold:

- `BACKGROUND_JOBS_ENABLED=true` (the default);
- the engine is not `postgres`, and there are embeddings or an external engine (`qdrant`,
  `meilisearch`);
- the project already has collected documentation snapshots (the project's Documentation tab,
  `KNOWLEDGE_INTERVAL_SECS`).

Every 5 seconds the job claims the projects that are due and does this:

1. **Embeddings.** Files of the latest snapshots of every branch are cut into fragments: by
   Markdown headings, no longer than `KNOWLEDGE_CHUNK_CHARS`. Content that has no vectors for the
   current model yet is embedded in batches of `EMBEDDINGS_BATCH`. The same content in different
   files, branches and projects is embedded once.
2. **External engine.** The whole project is sent to the external engine when its fingerprint
   changes. The fingerprint covers the files, the engine with its address and index or collection,
   and the model. Documents of vanished files and branches are removed; documents of deleted
   projects are removed once an hour.
3. **Next check** of the project — after `KNOWLEDGE_INDEX_INTERVAL_SECS`.

Changing `EMBEDDINGS_MODEL`, the engine, `QDRANT_COLLECTION` or `MEILISEARCH_INDEX` reindexes
everything by itself. Vectors of the old model stay in `knowledge_embeddings` and are reused if
you switch back.

If the embeddings API or the engine is down, files stay queued and are retried at the next check.
Documentation collection and the `text` mode of `pgvector` and `qdrant` keep working. Modes that
depend on the failed component answer `503 search.unavailable`.

Every indexing run is recorded in the scan history (**Administration → Scans**, `/<locale>/admin/scans`)
with its counters and, on failure, one of these codes and the technical details:

| Code | Meaning |
|---|---|
| `search.embeddings_unavailable` | the embeddings API did not answer or answered with an error |
| `search.embeddings_dimensions` | the model returns vectors of another size than `EMBEDDINGS_DIMENSIONS` |
| `search.engine_unavailable` | Qdrant, Meilisearch or the `vector` extension did not answer |
| `search.engine_dimensions` | the Qdrant collection stores vectors of another size than `EMBEDDINGS_DIMENSIONS` |

The `serve` log carries the same code and details:
`documentation indexing failed project=<id> code=search.embeddings_dimensions detail="… vector 0 has 384
dimensions, want 768 (EMBEDDINGS_DIMENSIONS)"`. Secrets are removed from the details.

## 5. Checking that it works

1. **The `serve` startup log** has one line with the engine and whether indexing runs:
   ```
   level=INFO msg="documentation search" engine=qdrant modes="[text semantic hybrid]" default=hybrid embeddings_model=bge-m3 indexing=true
   ```
   `indexing=false` means the job did not start: the engine is `postgres` or
   `BACKGROUND_JOBS_ENABLED=false`.
2. **The indexing log** has a line for every project where work was done:
   ```
   level=INFO msg="documentation indexed" project=… engine=qdrant embedded_files=12 embedded_chunks=48 synced=true documents=48
   ```
   Passes with nothing to do show up with `LOG_LEVEL=debug`. Failures are logged at `WARN` as
   `documentation indexing failed`.
3. **The API:**
   ```sh
   curl -b cookies 'http://localhost:8080/api/v1/knowledge/search/modes'
   # {"engine":"qdrant","modes":["text","semantic","hybrid"],"default":"hybrid","index":{"pending":0,"indexed":57}}
   ```
   - `pending` — files still waiting for embeddings.
   - `index: null` — no embeddings configured.
4. **The UI** at `http://localhost:8080/en/search?q=…`.
   - The "Text / By meaning / Mixed" switch shows when the engine has more than one mode; the mode
     goes into the address (`&mode=semantic`).
   - While indexing is in progress, "Files still being indexed: N" is shown next to it.
5. **The engine itself:**
   - Qdrant — dashboard `http://localhost:6333/dashboard`, the collection from `QDRANT_COLLECTION`.
   - Meilisearch — `curl 'http://localhost:7700/indexes/svc_registry_docs/documents?limit=3'`.
   - pgvector:
     ```sql
     SELECT model, count(*) FROM knowledge_embeddings GROUP BY model;
     ```

## 6. Common problems

| Symptom | Cause | Fix |
|---|---|---|
| Qdrant/Meilisearch indexes are empty, nothing in the log | `KNOWLEDGE_SEARCH_ENGINE` is not set → `postgres` | set the engine, restart `serve` |
| `serve` fails: `EMBEDDINGS_URL … is required by KNOWLEDGE_SEARCH_ENGINE=qdrant` | `pgvector`/`qdrant` require embeddings | set `EMBEDDINGS_URL`, `EMBEDDINGS_MODEL`, `EMBEDDINGS_DIMENSIONS` |
| `QDRANT_URL must be host:port of the Qdrant gRPC API` | the address has a scheme | `QDRANT_URL=localhost:6334` |
| `semantic` answers `503`, a Qdrant error in the log | HTTP port 6333 instead of gRPC 6334, or Qdrant is unreachable | `QDRANT_URL=host:6334`, check the network |
| `serve` fails: `needs the PostgreSQL extension "vector"` | pgvector is not in the database | install the extension (see `pgvector`) |
| `pending` does not go down, log says `vector … has N dimensions, want M` | `EMBEDDINGS_DIMENSIONS` does not match the model | set the model's native size |
| `semantic` → `400 validation.search_mode_unavailable` | the engine has no such mode (`postgres`, or `meilisearch` without embeddings) | configure embeddings or search in `text` |
| Meilisearch answers `401`/`403` | the server has `MEILI_MASTER_KEY`, and `MEILISEARCH_API_KEY` is empty or lacks rights | set a key with the rights listed under `meilisearch` |

## 7. Complete examples

Word search only, nothing extra:
```sh
KNOWLEDGE_SEARCH_ENGINE=postgres
```

Search by meaning in the same database, local model:
```sh
KNOWLEDGE_SEARCH_ENGINE=pgvector
EMBEDDINGS_URL=http://ollama:11434/v1
EMBEDDINGS_MODEL=bge-m3
EMBEDDINGS_DIMENSIONS=1024
```

Qdrant with OpenAI:
```sh
KNOWLEDGE_SEARCH_ENGINE=qdrant
QDRANT_URL=qdrant:6334
QDRANT_API_KEY=file:/run/secrets/qdrant_api_key
QDRANT_COLLECTION=svc_registry_docs_te3s
EMBEDDINGS_URL=https://api.openai.com/v1
EMBEDDINGS_MODEL=text-embedding-3-small
EMBEDDINGS_DIMENSIONS=1536
EMBEDDINGS_API_KEY=env:OPENAI_API_KEY
```

Meilisearch for typo-tolerant text search only:
```sh
KNOWLEDGE_SEARCH_ENGINE=meilisearch
MEILISEARCH_URL=http://meilisearch:7700
MEILISEARCH_API_KEY=env:MEILI_ADMIN_KEY
```
