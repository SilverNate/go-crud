package postgres

import (
	"context"
	"errors"

	"github.com/go-crud/go-task-api/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct{ pool *pgxpool.Pool }

func New(ctx context.Context, databaseURL string) (*Repository, error) {
	if databaseURL == "" {
		return nil, errors.New("DATABASE_URL is required when STORAGE_DRIVER=postgres")
	}
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return &Repository{pool: pool}, nil
}

func (r *Repository) Close() { r.pool.Close() }

func (r *Repository) Create(ctx context.Context, task domain.Task) (domain.Task, error) {
	err := r.pool.QueryRow(ctx, `
		INSERT INTO tasks (title, description, completed, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id`, task.Title, task.Description, task.Completed, task.CreatedAt, task.UpdatedAt).Scan(&task.ID)
	return task, err
}

func (r *Repository) List(ctx context.Context) ([]domain.Task, error) {
	rows, err := r.pool.Query(ctx, `SELECT id, title, description, completed, created_at, updated_at FROM tasks ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	tasks := make([]domain.Task, 0)
	for rows.Next() {
		var task domain.Task
		if err := rows.Scan(&task.ID, &task.Title, &task.Description, &task.Completed, &task.CreatedAt, &task.UpdatedAt); err != nil {
			return nil, err
		}
		tasks = append(tasks, task)
	}
	return tasks, rows.Err()
}

func (r *Repository) GetByID(ctx context.Context, id int64) (domain.Task, error) {
	var task domain.Task
	err := r.pool.QueryRow(ctx, `
		SELECT id, title, description, completed, created_at, updated_at
		FROM tasks WHERE id = $1`, id).Scan(&task.ID, &task.Title, &task.Description, &task.Completed, &task.CreatedAt, &task.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Task{}, domain.ErrNotFound
	}
	return task, err
}

func (r *Repository) Update(ctx context.Context, task domain.Task) (domain.Task, error) {
	err := r.pool.QueryRow(ctx, `
		UPDATE tasks SET title=$1, description=$2, completed=$3, updated_at=$4
		WHERE id=$5
		RETURNING id, title, description, completed, created_at, updated_at`,
		task.Title, task.Description, task.Completed, task.UpdatedAt, task.ID,
	).Scan(&task.ID, &task.Title, &task.Description, &task.Completed, &task.CreatedAt, &task.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Task{}, domain.ErrNotFound
	}
	return task, err
}

func (r *Repository) Delete(ctx context.Context, id int64) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM tasks WHERE id=$1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}
