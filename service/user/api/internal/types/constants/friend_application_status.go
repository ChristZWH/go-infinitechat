package constants

type FriendApplicationStatus struct {
	Code        int64
	Description string
}

var (
	ApplicationUnread   = FriendApplicationStatus{Code: 0, Description: "未读"}
	ApplicationAccepted = FriendApplicationStatus{Code: 1, Description: "通过"}
	ApplicationRejected = FriendApplicationStatus{Code: 2, Description: "拒绝"}
	ApplicationRead     = FriendApplicationStatus{Code: 3, Description: "已读"}
	ApplicationExpired  = FriendApplicationStatus{Code: 4, Description: "过期"}
)

// Redis Key 前缀(好友状态缓存)
const FriendStatusKeyPrefix = "msg:validate:friend:status:"
