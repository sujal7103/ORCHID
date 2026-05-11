package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Team represents a team for collaborative agent access
// NOTE: This is schema-only for now - team logic not yet implemented
type Team struct {
	ID      primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	Name    string             `bson:"name" json:"name"`
	OwnerID string             `bson:"ownerId" json:"ownerId"` // Supabase user ID

	// Team members
	Members []TeamMember `bson:"members" json:"members"`

	// Team settings
	Settings TeamSettings `bson:"settings" json:"settings"`

	CreatedAt time.Time `bson:"createdAt" json:"createdAt"`
	UpdatedAt time.Time `bson:"updatedAt" json:"updatedAt"`
}

// TeamMember represents a member of a team
type TeamMember struct {
	UserID  string    `bson:"userId" json:"userId"`
	Role    string    `bson:"role" json:"role"` // owner, admin, editor, viewer
	AddedAt time.Time `bson:"addedAt" json:"addedAt"`
	AddedBy string    `bson:"addedBy" json:"addedBy"`
}

// TeamSettings contains team configuration
type TeamSettings struct {
	DefaultAgentVisibility string `bson:"defaultAgentVisibility" json:"defaultAgentVisibility"` // private, team
}

// TeamRole constants
const (
	TeamRoleOwner  = "owner"
	TeamRoleAdmin  = "admin"
	TeamRoleEditor = "editor"
	TeamRoleViewer = "viewer"
)

// IsOwner checks if a user is the team owner
func (t *Team) IsOwner(userID string) bool {
	return t.OwnerID == userID
}

// GetMemberRole returns the role of a member (empty if not a member)
func (t *Team) GetMemberRole(userID string) string {
	if t.OwnerID == userID {
		return TeamRoleOwner
	}
	for _, m := range t.Members {
		if m.UserID == userID {
			return m.Role
		}
	}
	return ""
}

// HasMember checks if a user is a team member
func (t *Team) HasMember(userID string) bool {
	return t.GetMemberRole(userID) != ""
}

// CanManageTeam checks if a user can manage team settings
func (t *Team) CanManageTeam(userID string) bool {
	role := t.GetMemberRole(userID)
	return role == TeamRoleOwner || role == TeamRoleAdmin
}

// CanEditAgents checks if a user can edit team agents
func (t *Team) CanEditAgents(userID string) bool {
	role := t.GetMemberRole(userID)
	return role == TeamRoleOwner || role == TeamRoleAdmin || role == TeamRoleEditor
}

// CanViewAgents checks if a user can view team agents
func (t *Team) CanViewAgents(userID string) bool {
	return t.HasMember(userID)
}

// AgentPermission represents granular agent access permissions
// NOTE: This is schema-only for now - permission logic not yet implemented
type AgentPermission struct {
	ID      primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	AgentID string             `bson:"agentId" json:"agentId"`

	// Who has access (one of these should be set)
	TeamID primitive.ObjectID `bson:"teamId,omitempty" json:"teamId,omitempty"` // If shared with team
	UserID string             `bson:"userId,omitempty" json:"userId,omitempty"` // If shared with individual

	// What access level
	Permission string `bson:"permission" json:"permission"` // view, execute, edit, admin

	GrantedBy string    `bson:"grantedBy" json:"grantedBy"`
	GrantedAt time.Time `bson:"grantedAt" json:"grantedAt"`
}

// Permission level constants
const (
	PermissionView    = "view"    // Can view agent and executions
	PermissionExecute = "execute" // Can execute agent
	PermissionEdit    = "edit"    // Can edit agent workflow
	PermissionAdmin   = "admin"   // Full control including sharing
)

// CanView checks if the permission allows viewing
func (p *AgentPermission) CanView() bool {
	return p.Permission == PermissionView ||
		p.Permission == PermissionExecute ||
		p.Permission == PermissionEdit ||
		p.Permission == PermissionAdmin
}

// CanExecute checks if the permission allows execution
func (p *AgentPermission) CanExecute() bool {
	return p.Permission == PermissionExecute ||
		p.Permission == PermissionEdit ||
		p.Permission == PermissionAdmin
}

// CanEdit checks if the permission allows editing
func (p *AgentPermission) CanEdit() bool {
	return p.Permission == PermissionEdit ||
		p.Permission == PermissionAdmin
}

// CanAdmin checks if the permission allows admin access
func (p *AgentPermission) CanAdmin() bool {
	return p.Permission == PermissionAdmin
}

// AgentVisibility constants for agent model extension
const (
	VisibilityPrivate = "private" // Only owner
	VisibilityTeam    = "team"    // Team members
	VisibilityPublic  = "public"  // Anyone (future)
)

// CreateTeamRequest is the request body for creating a team
type CreateTeamRequest struct {
	Name string `json:"name"`
}

// InviteMemberRequest is the request body for inviting a team member
type InviteMemberRequest struct {
	Email string `json:"email"` // User email to invite
	Role  string `json:"role"`  // Role to assign
}

// ShareAgentRequest is the request body for sharing an agent
type ShareAgentRequest struct {
	TeamID     string `json:"teamId,omitempty"`     // Share with team
	UserID     string `json:"userId,omitempty"`     // Share with user
	Permission string `json:"permission"`           // Permission level
}

// TeamListItem is a lightweight team representation
type TeamListItem struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Role        string `json:"role"` // Current user's role
	MemberCount int    `json:"memberCount"`
}
