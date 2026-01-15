// Package session 会话状态管理
package session

import (
	"sync"
	"time"
)

// State 会话状态类型
type State string

const (
	StateNone        State = ""
	StateWaitingCode State = "waiting_code" // 等待输入注册码
	StateWaitingName State = "waiting_name" // 等待输入用户名

	// MoviePilot 点播相关状态
	StateMoviePilotSearch    State = "moviepilot_search"     // 等待输入搜索关键词
	StateMoviePilotSelectMedia State = "moviepilot_select_media" // 等待选择媒体
	StateMoviePilotConfirm   State = "moviepilot_confirm"    // 等待确认下载
)

// UserSession 用户会话
type UserSession struct {
	State     State
	Data      map[string]interface{}
	UpdatedAt time.Time
}

// Manager 会话管理器
type Manager struct {
	sessions map[int64]*UserSession
	mu       sync.RWMutex
	ttl      time.Duration
}

var (
	instance *Manager
	once     sync.Once
)

// GetManager 获取会话管理器单例
func GetManager() *Manager {
	once.Do(func() {
		instance = &Manager{
			sessions: make(map[int64]*UserSession),
			ttl:      10 * time.Minute, // 会话超时时间
		}

		// 启动清理协程
		go instance.cleanup()
	})
	return instance
}

// SetState 设置用户状态
func (m *Manager) SetState(userID int64, state State) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if session, ok := m.sessions[userID]; ok {
		session.State = state
		session.UpdatedAt = time.Now()
	} else {
		m.sessions[userID] = &UserSession{
			State:     state,
			Data:      make(map[string]interface{}),
			UpdatedAt: time.Now(),
		}
	}
}

// GetState 获取用户状态
func (m *Manager) GetState(userID int64) State {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if session, ok := m.sessions[userID]; ok {
		return session.State
	}
	return StateNone
}

// SetData 设置会话数据
func (m *Manager) SetData(userID int64, key string, value interface{}) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if session, ok := m.sessions[userID]; ok {
		session.Data[key] = value
		session.UpdatedAt = time.Now()
	} else {
		m.sessions[userID] = &UserSession{
			State:     StateNone,
			Data:      map[string]interface{}{key: value},
			UpdatedAt: time.Now(),
		}
	}
}

// GetData 获取会话数据
func (m *Manager) GetData(userID int64, key string) (interface{}, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if session, ok := m.sessions[userID]; ok {
		val, exists := session.Data[key]
		return val, exists
	}
	return nil, false
}

// ClearSession 清除用户会话
func (m *Manager) ClearSession(userID int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.sessions, userID)
}

// cleanup 定期清理过期会话
func (m *Manager) cleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		m.mu.Lock()
		now := time.Now()
		for userID, session := range m.sessions {
			if now.Sub(session.UpdatedAt) > m.ttl {
				delete(m.sessions, userID)
			}
		}
		m.mu.Unlock()
	}
}
