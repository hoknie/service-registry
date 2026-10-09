package config

type PasswordHashConfig struct {
	MemoryKiB   uint32 `env:"PASSWORD_HASH_MEMORY_KIB" envDefault:"19456" validate:"min=1024,max=1048576"`
	Iterations  uint32 `env:"PASSWORD_HASH_ITERATIONS" envDefault:"2" validate:"min=1,max=10"`
	Parallelism uint32 `env:"PASSWORD_HASH_PARALLELISM" envDefault:"1" validate:"min=1,max=16"`
}
