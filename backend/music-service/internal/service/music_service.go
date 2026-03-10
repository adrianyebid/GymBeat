package service

import (
"fmt"
"strings"
"sync/atomic"
"time"

"github.com/adrianyebid/fitbeat/music-service/internal/model"
"github.com/adrianyebid/fitbeat/music-service/internal/repository"
)

var ErrSessionNotFound = fmt.Errorf("session not found")

type CreateSessionInput struct {
UserID       string
ActivityType string
Mode         string
}

// EngineService implementa la logica de creacion y consulta de sesiones de entrenamiento.
type EngineService struct {
repo      repository.EngineRepository
idCounter uint64
}

func NewEngineService(repo repository.EngineRepository) *EngineService {
return &EngineService{repo: repo}
}

// CreateSession crea y persiste una nueva sesion de entrenamiento.
func (s *EngineService) CreateSession(input CreateSessionInput) (model.TrainingSession, error) {
session := model.TrainingSession{
ID:           s.nextID("session"),
UserID:       strings.TrimSpace(input.UserID),
ActivityType: strings.TrimSpace(input.ActivityType),
Mode:         strings.TrimSpace(input.Mode),
CreatedAt:    time.Now().UTC(),
}

if err := s.repo.SaveSession(session); err != nil {
return model.TrainingSession{}, err
}

return session, nil
}

func (s *EngineService) nextID(prefix string) string {
n := atomic.AddUint64(&s.idCounter, 1)
return fmt.Sprintf("%s_%d_%d", prefix, time.Now().UTC().UnixNano(), n)
}
