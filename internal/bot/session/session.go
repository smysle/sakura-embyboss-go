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

	// 用户创建相关状态
	StateWaitingCreateInfo State = "waiting_create_info" // 等待输入用户名和安全码

	// 安全验证相关状态
	StateWaitingSecurityCode     State = "waiting_security_code"      // 等待输入安全码验证
	StateWaitingNewPassword      State = "waiting_new_password"       // 等待输入新密码
	StateWaitingDeleteConfirm    State = "waiting_delete_confirm"     // 等待删除确认

	// 换绑TG相关状态
	StateWaitingChangeTGInfo State = "waiting_changetg_info" // 等待输入换绑信息
	StateWaitingBindTGInfo   State = "waiting_bindtg_info"   // 等待输入绑定信息

	// MoviePilot 点播相关状态
	StateMoviePilotSearch      State = "moviepilot_search"       // 等待输入搜索关键词
	StateMoviePilotSelectMedia State = "moviepilot_select_media" // 等待选择媒体
	StateMoviePilotConfirm     State = "moviepilot_confirm"      // 等待确认下载

	// 配置面板相关状态
	StateWaitingInput State = "waiting_input" // 等待用户输入（配置面板通用）
)

// ActionType 操作类型（用于安全码验证后的操作）
type ActionType string

const (
	ActionNone         ActionType = ""
	ActionResetPwd     ActionType = "reset_pwd"     // 重置密码
	ActionDeleteAccount ActionType = "delete_account" // 删除账户
	ActionChangeTG     ActionType = "change_tg"     // 换绑TG
)

// UserSession 用户会话
type UserSession struct {
	State       State
	Action      ActionType             // 当前操作类型
	Data        map[string]interface{}
	UpdatedAt   time.Time
	MessageID   int                    // 记录消息ID，用于编辑
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
			ttl:      5 * time.Minute, // 会话超时时间缩短到5分钟
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

// SetStateWithAction 设置用户状态和操作类型
func (m *Manager) SetStateWithAction(userID int64, state State, action ActionType) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if session, ok := m.sessions[userID]; ok {
		session.State = state
		session.Action = action
		session.UpdatedAt = time.Now()
	} else {
		m.sessions[userID] = &UserSession{
			State:     state,
			Action:    action,
			Data:      make(map[string]interface{}),
			UpdatedAt: time.Now(),
		}
	}
}

// SetStateWithStringAction 设置用户状态和字符串操作类型（用于配置面板等动态 action）
func (m *Manager) SetStateWithStringAction(userID int64, state State, action string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if session, ok := m.sessions[userID]; ok {
		session.State = state
		session.Data["string_action"] = action
		session.UpdatedAt = time.Now()
	} else {
		m.sessions[userID] = &UserSession{
			State:     state,
			Data:      map[string]interface{}{"string_action": action},
			UpdatedAt: time.Now(),
		}
	}
}

// GetStringAction 获取字符串操作类型
func (m *Manager) GetStringAction(userID int64) string {
	val, ok := m.GetData(userID, "string_action")
	if !ok {
		return ""
	}
	if s, ok := val.(string); ok {
		return s
	}
	return ""
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

// GetAction 获取用户当前操作类型
func (m *Manager) GetAction(userID int64) ActionType {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if session, ok := m.sessions[userID]; ok {
		return session.Action
	}
	return ActionNone
}

// GetSession 获取完整的会话信息
func (m *Manager) GetSession(userID int64) *UserSession {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if session, ok := m.sessions[userID]; ok {
		return session
	}
	return nil
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

// GetDataString 获取字符串类型的会话数据
func (m *Manager) GetDataString(userID int64, key string) string {
	val, ok := m.GetData(userID, key)
	if !ok {
		return ""
	}
	if s, ok := val.(string); ok {
		return s
	}
	return ""
}

// GetDataInt 获取整数类型的会话数据
func (m *Manager) GetDataInt(userID int64, key string) int {
	val, ok := m.GetData(userID, key)
	if !ok {
		return 0
	}
	if i, ok := val.(int); ok {
		return i
	}
	return 0
}

// SetMessageID 设置消息ID
func (m *Manager) SetMessageID(userID int64, msgID int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if session, ok := m.sessions[userID]; ok {
		session.MessageID = msgID
		session.UpdatedAt = time.Now()
	}
}

// GetMessageID 获取消息ID
func (m *Manager) GetMessageID(userID int64) int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if session, ok := m.sessions[userID]; ok {
		return session.MessageID
	}
	return 0
}

// ClearSession 清除用户会话
func (m *Manager) ClearSession(userID int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.sessions, userID)
}

// ClearState 清除用户状态（ClearSession 的别名）
func (m *Manager) ClearState(userID int64) {
	m.ClearSession(userID)
}

// HasActiveSession 检查用户是否有活跃会话
func (m *Manager) HasActiveSession(userID int64) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	session, ok := m.sessions[userID]
	if !ok {
		return false
	}
	return session.State != StateNone
}

// cleanup 定期清理过期会话
func (m *Manager) cleanup() {
	ticker := time.NewTicker(1 * time.Minute)
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
