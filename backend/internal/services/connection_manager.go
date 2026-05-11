package services

import (
	"clara-agents/internal/models"
	"log"
	"sync"
)

// ConnectionManager manages all active WebSocket connections
type ConnectionManager struct {
	connections map[string]*models.UserConnection
	mutex       sync.RWMutex
}

// NewConnectionManager creates a new connection manager
func NewConnectionManager() *ConnectionManager {
	return &ConnectionManager{
		connections: make(map[string]*models.UserConnection),
	}
}

// Add adds a new connection
func (cm *ConnectionManager) Add(conn *models.UserConnection) {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()
	cm.connections[conn.ConnID] = conn
	log.Printf("✅ Connection added: %s (Total: %d)", conn.ConnID, len(cm.connections))
}

// Remove removes a connection
func (cm *ConnectionManager) Remove(connID string) {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()
	if conn, exists := cm.connections[connID]; exists {
		close(conn.WriteChan)
		close(conn.StopChan)
		delete(cm.connections, connID)
		log.Printf("❌ Connection removed: %s (Total: %d)", connID, len(cm.connections))
	}
}

// Get retrieves a connection by ID
func (cm *ConnectionManager) Get(connID string) (*models.UserConnection, bool) {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()
	conn, exists := cm.connections[connID]
	return conn, exists
}

// Count returns the number of active connections
func (cm *ConnectionManager) Count() int {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()
	return len(cm.connections)
}

// GetAll returns all active connections
func (cm *ConnectionManager) GetAll() []*models.UserConnection {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()

	conns := make([]*models.UserConnection, 0, len(cm.connections))
	for _, conn := range cm.connections {
		conns = append(conns, conn)
	}
	return conns
}
