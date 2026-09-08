package dto

import "time"

type CreateGroupResponse struct {
	SessionId       string   `json:"sessionId"`
	SessionName     string   `json:"sessionName"`
	SessionType     int      `json:"sessionType"`
	Avatar          string   `json:"avatar"`
	CreatorId       string   `json:"creatorId"`
	MembersCount    int      `json:"membersCount"`
	FailedMemberIds []string `json:"failedMemberIds"`
}

type GroupMemberDTO struct {
	UserId   string `json:"userId"`
	Nickname string `json:"nickname"`
	Avatar   string `json:"avatar"`
	Role     int64  `json:"role"`
}

type UserGroupDTO struct {
	SessionId   string `json:"sessionId"`
	SessionName string `json:"sessionName"`
	Avatar      string `json:"avatar"`
	CreatorId   string `json:"creatorId"`
	Role        int64  `json:"role"`
	MemberCount int    `json:"memberCount"`
	CreatedTime string `json:"createdTime"`
}

type GroupRow struct {
	SessionId   int64     `db:"session_id"`
	Role        int64     `db:"role"`
	CreatedTime time.Time `db:"created_time"`
	Name        string    `db:"name"`
	Avatar      string    `db:"avatar"`
	OwnerId     int64     `db:"owner_id"`
	MemberCount int64     `db:"member_count"`
}
