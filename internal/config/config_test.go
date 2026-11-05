package config

import "testing"

func TestLoadDefaults(t *testing.T) {
	for _, k := range []string{"HTTP_ADDR", "STORAGE_DRIVER", "DATABASE_URL", "LOG_LEVEL", "WORKER_QUEUE_SIZE"} {
		t.Setenv(k, "") // empty == unset for this package
	}
	cfg := Load()
	want := Config{HTTPAddr: ":8080", StorageDriver: "memory", LogLevel: "debug", WorkerQueueSize: 64}
	if cfg != want {
		t.Fatalf("Load() = %+v, want %+v", cfg, want)
	}
}

func TestLoadFromEnvironment(t *testing.T) {
	t.Setenv("HTTP_ADDR", ":9090")
	t.Setenv("STORAGE_DRIVER", "postgres")
	t.Setenv("DATABASE_URL", "postgres://u:p@db:5432/x")
	t.Setenv("LOG_LEVEL", "info")
	t.Setenv("WORKER_QUEUE_SIZE", "5")
	cfg := Load()
	want := Config{":9090", "postgres", "postgres://u:p@db:5432/x", "info", 5}
	if cfg != want {
		t.Fatalf("Load() = %+v, want %+v", cfg, want)
	}
}

func TestInvalidQueueSizeFallsBack(t *testing.T) {
	for _, v := range []string{"abc", "0", "-3", "1.5"} {
		t.Setenv("WORKER_QUEUE_SIZE", v)
		if got := Load().WorkerQueueSize; got != 64 {
			t.Fatalf("WORKER_QUEUE_SIZE=%q -> %d, want fallback 64", v, got)
		}
	}
}
