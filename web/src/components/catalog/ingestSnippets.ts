export type Snippets = { endpoint: string; curl: string; github: string; gitlab: string };

export function ingestPath(projectId: string): string {
  return `/api/v1/ingest/projects/${projectId}/events`;
}

function envelope(key: string, payload: string[], indent: string): string {
  return [
    "{",
    '  "type": "service.deployed",',
    '  "version": 1,',
    '  "occurred_at": "$OCCURRED_AT",',
    `  "idempotency_key": "${key}",`,
    '  "payload": {',
    ...payload.map((line, i) => `    ${line}${i < payload.length - 1 ? "," : ""}`),
    "  }",
    "}",
  ]
    .map((line) => indent + line)
    .join("\n");
}

function post(projectId: string, indent: string): string {
  return [
    `curl --fail-with-body --retry 3 --retry-all-errors -X POST \\`,
    `  "$SVCR_URL${ingestPath(projectId)}" \\`,
    `  -H "Authorization: Bearer $SVCR_PROJECT_KEY" \\`,
    `  -H "Content-Type: application/json" \\`,
    `  --data @- <<EOF`,
  ]
    .map((line) => indent + line)
    .join("\n");
}

export function snippets(origin: string, projectId: string): Snippets {
  const curl = [
    `export SVCR_URL="${origin}"`,
    `OCCURRED_AT="$(date -u +%Y-%m-%dT%H:%M:%SZ)"`,
    post(projectId, ""),
    envelope(
      "deploy-my-service-production-$OCCURRED_AT",
      [
        '"service": "my-service"',
        '"version": "1.4.2"',
        '"environment": "production"',
        '"commit_sha": "$(git rev-parse HEAD)"',
        '"branch": "$(git rev-parse --abbrev-ref HEAD)"',
      ],
      "",
    ),
    "EOF",
  ].join("\n");

  const github = [
    "- name: Register the deployment in svc-registry",
    "  env:",
    "    SVCR_URL: ${{ vars.SVCR_URL }}",
    "    SVCR_PROJECT_KEY: ${{ secrets.SVCR_PROJECT_KEY }}",
    "  run: |",
    '    OCCURRED_AT="$(date -u +%Y-%m-%dT%H:%M:%SZ)"',
    post(projectId, "    "),
    envelope(
      "gh-${{ github.run_id }}-${{ github.run_attempt }}-production",
      [
        '"service": "my-service"',
        '"version": "${{ github.sha }}"',
        '"environment": "production"',
        '"commit_sha": "${{ github.sha }}"',
        '"branch": "${{ github.ref_name }}"',
        '"url": "https://my-service.example.com"',
        '"deployed_by": "${{ github.actor }}"',
        '"metadata": { "run_url": "${{ github.server_url }}/${{ github.repository }}/actions/runs/${{ github.run_id }}" }',
      ],
      "    ",
    ),
    "    EOF",
  ].join("\n");

  const gitlab = [
    "register-deployment:",
    "  stage: deploy",
    "  image: curlimages/curl:8.11.1",
    "  needs: [deploy-production]",
    "  script:",
    "    - |",
    '      OCCURRED_AT="$(date -u +%Y-%m-%dT%H:%M:%SZ)"',
    post(projectId, "      "),
    envelope(
      "gl-$CI_PIPELINE_ID-$CI_JOB_ID-production",
      [
        '"service": "my-service"',
        '"version": "$CI_COMMIT_SHORT_SHA"',
        '"environment": "production"',
        '"commit_sha": "$CI_COMMIT_SHA"',
        '"branch": "$CI_COMMIT_REF_NAME"',
        '"deployed_by": "$GITLAB_USER_LOGIN"',
        '"metadata": { "pipeline_url": "$CI_PIPELINE_URL" }',
      ],
      "      ",
    ),
    "      EOF",
  ].join("\n");

  return { endpoint: origin + ingestPath(projectId), curl, github, gitlab };
}
