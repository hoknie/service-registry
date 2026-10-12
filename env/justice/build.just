# Build every package (compile check)
[group('build')]
build:
    {{ GO }} build ./...

# Release binary into bin/ (static: CGO_ENABLED=0, stripped, reproducible paths)
[group('build')]
build-release:
    mkdir -p bin
    CGO_ENABLED=0 {{ GO }} build -trimpath -ldflags='-s -w' -o bin/svc-registry ./cmd/svc-registry
    ls -l bin/svc-registry
