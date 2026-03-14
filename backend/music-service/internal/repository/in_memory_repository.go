package repository

import (
	"sync"

	"github.com/adrianyebid/fitbeat/music-service/internal/model"
)

// InMemoryRepository implementa EngineRepository usando almacenamiento en memoria.
type InMemoryRepository struct {
	mu       sync.RWMutex
	sessions map[string]model.TrainingSession
}

func NewInMemoryRepository() *InMemoryRepository {
	return &InMemoryRepository{
		sessions: make(map[string]model.TrainingSession),
	}
}

func (r *InMemoryRepository) SaveSession(session model.TrainingSession) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.sessions[session.ID] = session
	return nil
}
