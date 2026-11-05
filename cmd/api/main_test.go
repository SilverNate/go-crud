package main

import (
	"testing"

	"github.com/go-crud/go-task-api/internal/config"
)

func TestBuildRepositoryRejectsUnknownDriver(t *testing.T) {
	if _, _, err := buildRepository(config.Config{StorageDriver: "mongo"}); err == nil {
		t.Fatal("want error for unsupported STORAGE_DRIVER")
	}
}

func TestBuildRepositoryPostgresRequiresURL(t *testing.T) {
	if _, _, err := buildRepository(config.Config{StorageDriver: "postgres"}); err == nil {
		t.Fatal("want error when DATABASE_URL is empty")
	}
}
