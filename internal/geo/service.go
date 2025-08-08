package geo

import (
    "context"
    "encoding/json"
    "errors"
    "sync"

    "github.com/redis/go-redis/v9"

    kd "github.com/albus-droid/Capstone-Project-Backend/internal/algorithms/kd-tree"
)

const redisPointsKey = "kdtree:points"

// Service manages a KD-tree built from geospatial points persisted in Redis.
type Service struct {
    rdb  *redis.Client
    mu   sync.RWMutex
    tree *kd.KDTree
}

// NewService constructs a Service with its own Redis client.
func NewService(addr, password string, db int) *Service {
    r := redis.NewClient(&redis.Options{Addr: addr, Password: password, DB: db})
    return &Service{rdb: r}
}

// SavePoints overwrites the point set in Redis and rebuilds the KD-tree.
func (s *Service) SavePoints(ctx context.Context, points []kd.Point) error {
    raw, err := json.Marshal(points)
    if err != nil {
        return err
    }
    if err := s.rdb.Set(ctx, redisPointsKey, raw, 0).Err(); err != nil {
        return err
    }
    s.mu.Lock()
    s.tree = kd.NewFromPoints(points)
    s.mu.Unlock()
    return nil
}

// LoadOrBuild loads points from Redis and builds the KD-tree if not already loaded.
func (s *Service) LoadOrBuild(ctx context.Context) error {
    s.mu.RLock()
    if s.tree != nil {
        s.mu.RUnlock()
        return nil
    }
    s.mu.RUnlock()

    // Load from Redis
    raw, err := s.rdb.Get(ctx, redisPointsKey).Bytes()
    if err != nil {
        if errors.Is(err, redis.Nil) {
            return errors.New("no points stored in Redis")
        }
        return err
    }
    var pts []kd.Point
    if err := json.Unmarshal(raw, &pts); err != nil {
        return err
    }
    s.mu.Lock()
    s.tree = kd.NewFromPoints(pts)
    s.mu.Unlock()
    return nil
}

// Query returns all points within radiusKm of the given lon/lat. If the KD-tree
// is not built yet, it will be loaded from Redis.
func (s *Service) Query(ctx context.Context, lon, lat, radiusKm float64) ([]kd.Point, error) {
    if err := s.LoadOrBuild(ctx); err != nil {
        return nil, err
    }
    s.mu.RLock()
    t := s.tree
    s.mu.RUnlock()
    if t == nil {
        return nil, errors.New("kdtree not available")
    }
    return t.RangeSearchKm(lon, lat, radiusKm), nil
}