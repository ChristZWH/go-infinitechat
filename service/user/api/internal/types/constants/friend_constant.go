package constants

type FriendStatus struct {
	Code        int64
	Description string
}

func NewFriendStatus(code int64, description string) FriendStatus {
	return FriendStatus{
		Code:        code,
		Description: description,
	}
}

var (
	Normal    = NewFriendStatus(0, "好友")
	Blocked   = NewFriendStatus(1, "拉黑")
	Deleted   = NewFriendStatus(2, "删除")
	NonFriend = NewFriendStatus(-1, "非好友")
)

const (
	//单聊
	SignalType = iota
	//群聊
	MessageType
	//机器人
	NoticeType
)

const (
	//正常
	StatusSuccess = 0
	//异常
	StatusError = 1

	//会话用户角色常量
	UserRoleNormal = 2
)
