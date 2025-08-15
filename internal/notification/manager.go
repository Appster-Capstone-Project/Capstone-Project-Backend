package notification

import (
	"encoding/json"
	"io"
	"sync"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/albus-droid/Capstone-Project-Backend/internal/auth"
	"github.com/albus-droid/Capstone-Project-Backend/internal/event"
	"github.com/albus-droid/Capstone-Project-Backend/internal/order"
	"github.com/albus-droid/Capstone-Project-Backend/internal/seller"
)

type Manager struct {
	db      *gorm.DB
	mu      sync.Mutex
	sellers map[string][]chan event.Event // sellerID -> channels
	users   map[string][]chan event.Event // userEmail -> channels
}

func NewManager(db *gorm.DB) *Manager {
	return &Manager{
		db:      db,
		sellers: make(map[string][]chan event.Event),
		users:   make(map[string][]chan event.Event),
	}
}

func (m *Manager) Run() {
	for e := range event.Bus {
		m.mu.Lock()
		switch e.Type {
		case "OrderPlaced":
			o := e.Data.(order.Order)
			for _, ch := range m.sellers[o.SellerID] {
				ch <- e
			}
		case "OrderAccepted":
			o := e.Data.(order.Order)
			for _, ch := range m.users[o.UserEmail] {
				ch <- e
			}
		}
		m.mu.Unlock()
	}
}

func (m *Manager) RegisterRoutes(r *gin.Engine) {
	grp := r.Group("/notifications")
	grp.Use(auth.Middleware())
	grp.GET("", m.handle)
}

func (m *Manager) handle(c *gin.Context) {
	email := c.GetString(string(auth.CtxEmailKey))

	var key string
	var isSeller bool
	var sl seller.Seller
	if err := m.db.First(&sl, "email = ?", email).Error; err == nil {
		key = sl.ID
		isSeller = true
	} else {
		key = email
	}

	ch := make(chan event.Event, 10)

	m.mu.Lock()
	if isSeller {
		m.sellers[key] = append(m.sellers[key], ch)
	} else {
		m.users[key] = append(m.users[key], ch)
	}
	m.mu.Unlock()

	// Clean up when done
	defer func() {
		m.mu.Lock()
		if isSeller {
			arr := m.sellers[key]
			for i, cch := range arr {
				if cch == ch {
					m.sellers[key] = append(arr[:i], arr[i+1:]...)
					break
				}
			}
		} else {
			arr := m.users[key]
			for i, cch := range arr {
				if cch == ch {
					m.users[key] = append(arr[:i], arr[i+1:]...)
					break
				}
			}
		}
		m.mu.Unlock()
		close(ch)
	}()

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")

	c.Stream(func(w io.Writer) bool {
		select {
		case ev := <-ch:
			b, _ := json.Marshal(ev.Data)
			c.SSEvent(ev.Type, string(b))
			return true
		case <-c.Request.Context().Done():
			return false
		}
	})
}
