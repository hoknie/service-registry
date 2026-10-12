# Build and run the whole stack in containers (Postgres, migrate, the one image on :8080)
[group('docker')]
docker-up:
    docker compose up --build

[group('docker')]
docker-vdb ACTION='up' *ENGINES='all':
    #!/usr/bin/env bash
    set -euo pipefail
    services=()
    add() { case " ${services[*]-} " in *" $1 "*) ;; *) services+=("$1") ;; esac; }
    for e in {{ ENGINES }}; do
      case "$e" in
        all) add qdrant; add meilisearch ;;
        qdrant) add qdrant ;;
        meilisearch|meili) add meilisearch ;;
        *) echo "docker-vdb: unknown engine '$e' (qdrant, meilisearch, all)" >&2; exit 2 ;;
      esac
    done
    dc() { docker compose --profile search "$@"; }
    case "{{ ACTION }}" in
      up)
        dc up -d --wait "${services[@]}"
        echo
        for s in "${services[@]}"; do
          case "$s" in
            qdrant) echo "qdrant:      KNOWLEDGE_SEARCH_ENGINE=qdrant QDRANT_URL=localhost:${QDRANT_PORT:-6334}  (dashboard http://localhost:${QDRANT_HTTP_PORT:-6333}/dashboard)" ;;
            meilisearch) echo "meilisearch: KNOWLEDGE_SEARCH_ENGINE=meilisearch MEILISEARCH_URL=http://localhost:${MEILISEARCH_PORT:-7700}" ;;
          esac
        done
        echo "semantic/hybrid modes also need EMBEDDINGS_URL, EMBEDDINGS_MODEL, EMBEDDINGS_DIMENSIONS (see .env.example)" ;;
      down) dc stop "${services[@]}" && dc rm -f "${services[@]}" ;;
      restart) dc restart "${services[@]}" ;;
      logs) dc logs -f --tail 100 "${services[@]}" ;;
      ps) dc ps "${services[@]}" ;;
      reset)
        dc rm -sf "${services[@]}"
        for s in "${services[@]}"; do
          case "$s" in qdrant) v=qdrant-data ;; meilisearch) v=meili-data ;; esac
          docker volume ls -q --filter "label=com.docker.compose.project=svc-registry" \
            --filter "label=com.docker.compose.volume=$v" | xargs -r docker volume rm >/dev/null
        done
        dc up -d --wait "${services[@]}" ;;
      *) echo "docker-vdb: unknown action '{{ ACTION }}' (up, down, restart, logs, ps, reset)" >&2; exit 2 ;;
    esac
