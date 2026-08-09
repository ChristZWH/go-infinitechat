CREATE DATABASE IF NOT EXISTS `infiniteChat` CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci;

USE `infiniteChat`;

CREATE TABLE `apply_friend` (
    `apply_friend_id` bigint NOT NULL COMMENT '申请 ID',
    `sender_id` bigint NOT NULL COMMENT '发送者用户 ID',
    `receiver_id` bigint NOT NULL COMMENT '接收者用户 ID',
    `message` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci NOT NULL DEFAULT '请求添加好友' COMMENT '申请信息',
    `status` tinyint NOT NULL DEFAULT 0 COMMENT '申请状态。0未读，1通过，2拒绝，3已读，4过期',
    `created_time` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `updated_time` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',

    PRIMARY KEY (`apply_friend_id`) USING BTREE,
    UNIQUE INDEX `uk_sender_receiver`(`sender_id` ASC, `receiver_id` ASC) USING BTREE,
    INDEX `idx_sender_status`(`sender_id` ASC, `status` ASC) USING BTREE,
    INDEX `idx_receiver_status`(`receiver_id` ASC, `status` ASC) USING BTREE,
    CONSTRAINT `chk_apply_not_self` CHECK (`sender_id` <> `receiver_id`)

) ENGINE = InnoDB CHARACTER SET = utf8mb4 COLLATE = utf8mb4_0900_ai_ci COMMENT = '好友申请表' ROW_FORMAT = DYNAMIC;

CREATE TABLE `balance_log` (
    `balance_log_id` bigint NOT NULL COMMENT '记录 ID',
    `user_id` bigint NOT NULL COMMENT '用户 ID',
    `amount` bigint NOT NULL COMMENT '变动金额（单位：分），正数为增加，负数为减少',
    `type` tinyint NOT NULL COMMENT '变动类型：0 发送红包，1 领取红包，2 红包退回',
    `related_id` bigint NULL DEFAULT NULL COMMENT '关联 ID，如红包 ID',
    `created_time` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `updated_time` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',

    PRIMARY KEY (`balance_log_id`) USING BTREE,
    INDEX `idx_related_id`(`related_id` ASC) USING BTREE COMMENT '做账单回测/对账',
    INDEX `idx_user_id`(`user_id` ASC) USING BTREE COMMENT '查询个人日志'

) ENGINE = InnoDB CHARACTER SET = utf8mb4 COLLATE = utf8mb4_0900_ai_ci COMMENT = '余额变动记录表' ROW_FORMAT = DYNAMIC;

CREATE TABLE `friend` (
    `id` bigint NOT NULL AUTO_INCREMENT COMMENT '主键',
    `user_id` bigint NOT NULL COMMENT '用户 ID',
    `friend_id` bigint NOT NULL COMMENT '好友 ID',
    `status` tinyint NOT NULL DEFAULT 0 COMMENT '好友状态：0好友，1拉黑，2删除',
    `created_time` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `updated_time` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',

    PRIMARY KEY (`id`),
    UNIQUE INDEX (`user_id`, `friend_id`) USING BTREE,
    INDEX `idx_friend_id`(`friend_id` ASC) USING BTREE,
    CONSTRAINT `chk_friend_not_self` CHECK (`user_id` <> `friend_id`)

) ENGINE = InnoDB CHARACTER SET = utf8mb4 COLLATE = utf8mb4_0900_ai_ci COMMENT = '好友表' ROW_FORMAT = DYNAMIC;

CREATE TABLE `session` (
    `session_id` bigint NOT NULL COMMENT '会话 ID',
    `name` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci NOT NULL COMMENT '名称',
    `type` tinyint NOT NULL COMMENT '类别：0 单聊，1 群聊，2 机器人',
    `status` tinyint NOT NULL COMMENT '状态：0 正常，1 删除',
    `created_time` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `updated_time` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    `avatar` varchar(512) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci NULL DEFAULT 'http://47.115.55.73:9000/infinite-chat/default_avatar.png' COMMENT '用户头像',
    PRIMARY KEY (`session_id`) USING BTREE
) ENGINE = InnoDB CHARACTER SET = utf8mb4 COLLATE = utf8mb4_0900_ai_ci COMMENT = '会话表' ROW_FORMAT = DYNAMIC;

CREATE TABLE `user` (
    `user_id` bigint NOT NULL COMMENT '用户 ID (分布式 ID)',
    `phone` char(11) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci NULL DEFAULT NULL COMMENT '用户手机号',
    `email` varchar(128) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci NULL COMMENT '用户邮箱',
    `password` varchar(256) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci NULL COMMENT '用户密码',
    `nickname` varchar(128) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci NULL COMMENT '用户昵称',
    `avatar` varchar(512) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci NULL DEFAULT 'http://47.115.55.73:9000/infinite-chat/default_avatar.png' COMMENT '用户头像url',
    `gender` tinyint(1) NOT NULL DEFAULT 2 COMMENT '性别 0 女 1 男 2 未知',
    `description` text CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci NULL COMMENT '个性签名',
    `state` tinyint(1) NOT NULL DEFAULT 0 COMMENT '状态 0 正常 1 封禁 2 注销',
    `role` tinyint(1) NOT NULL DEFAULT 0 COMMENT '角色类型 0 普通用户 1 管理员 2 超级管理员',
    `created_time` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `updated_time` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    `is_delete` tinyint NOT NULL DEFAULT 0 COMMENT '删除标记 (0未删 1已删)',
    PRIMARY KEY (`user_id`) USING BTREE,
    UNIQUE INDEX idx_email(`email` ASC) USING BTREE,
    UNIQUE INDEX idx_phone(`phone` ASC) USING BTREE,
    INDEX idx_state(`state` ASC) USING BTREE,
    INDEX idx_role(`role` ASC) USING BTREE,
    INDEX idx_create_time(`created_time` ASC) USING BTREE
) ENGINE = InnoDB CHARACTER SET = utf8mb4 COLLATE = utf8mb4_0900_ai_ci COMMENT = '用户表' ROW_FORMAT = DYNAMIC;

CREATE TABLE `user_balance` (
    `user_id` bigint NOT NULL COMMENT '用户 ID',
    `balance` bigint NOT NULL DEFAULT 0 COMMENT '余额 (单位：分)',
    `created_time` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `updated_time` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    PRIMARY KEY (`user_id`) USING BTREE
) ENGINE = InnoDB CHARACTER SET = utf8mb4 COLLATE = utf8mb4_0900_ai_ci COMMENT = '用户余额表' ROW_FORMAT = DYNAMIC;

CREATE TABLE `user_session` (
    `user_id` bigint NOT NULL COMMENT '用户 id',
    `session_id` bigint NOT NULL COMMENT '会话 id',
    `role` tinyint NOT NULL COMMENT '角色：0 群主，1 管理员，2 普通用户',
    `status` tinyint NOT NULL COMMENT '状态：0 正常，1 删除',
    `created_time` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `updated_time` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    PRIMARY KEY (`user_id`, `session_id`) USING BTREE,
    INDEX `idx_session_status` (`session_id` ASC, `status` ASC) USING BTREE COMMENT '会话ID+状态复合索引，优化群成员数量查询'
) ENGINE = InnoDB CHARACTER SET = utf8mb4 COLLATE = utf8mb4_0900_ai_ci COMMENT = '用户会话关系表' ROW_FORMAT = DYNAMIC;

DROP TABLE IF EXISTS `red_packet`;
CREATE TABLE `red_packet` (
    `red_packet_id` bigint NOT NULL COMMENT '红包 ID',
    `sender_id` bigint NOT NULL COMMENT '发送者用户 ID',
    `session_id` bigint NOT NULL COMMENT '会话 ID（单聊或群聊）',
    `session_type` tinyint NOT NULL COMMENT '会话类型：0 单聊，1 群聊',
    `red_packet_wrapper_text` varchar(50) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci NOT NULL DEFAULT '恭喜发财，大吉大利' COMMENT '红包封面文案',
    `red_packet_type` tinyint NOT NULL COMMENT '红包类型：0 普通红包，1 拼手气红包',
    `total_amount` bigint NOT NULL COMMENT '红包总金额(单位：分)',
    `total_count` int NOT NULL COMMENT '红包总个数',
    `status` tinyint NOT NULL DEFAULT 0 COMMENT '状态：0 未领取完，1 已领取完，2 已过期',
    `created_time` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `updated_time` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    PRIMARY KEY (`red_packet_id`) USING BTREE,
    INDEX `idx_session_id` (`session_id` ASC) USING BTREE,
    INDEX `idx_sender_id` (`sender_id` ASC) USING BTREE
) ENGINE = InnoDB CHARACTER SET = utf8mb4 COLLATE = utf8mb4_0900_ai_ci COMMENT = '红包主表' ROW_FORMAT = DYNAMIC;

CREATE TABLE `red_packet_receive` (
    `red_packet_receive_id` bigint NOT NULL COMMENT '记录 ID',
    `red_packet_id` bigint NOT NULL COMMENT '红包 ID',
    `receiver_id` bigint NOT NULL COMMENT '领取者用户 ID',
    `amount` bigint NOT NULL COMMENT '领取金额（单位：分）',
    `received_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '领取时间',
    `created_time` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `updated_time` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    PRIMARY KEY (`red_packet_receive_id`) USING BTREE,
    INDEX `idx_receiver_id` (`receiver_id` ASC) USING BTREE,
    INDEX `idx_packet_id` (`red_packet_id` ASC) USING BTREE
) ENGINE = InnoDB CHARACTER SET = utf8mb4 COLLATE = utf8mb4_0900_ai_ci COMMENT = '红包领取记录表' ROW_FORMAT = DYNAMIC;

CREATE TABLE `message` (
    `message_id` bigint NOT NULL COMMENT '消息id',
    `sender_id` bigint NOT NULL COMMENT '发送者id',
    `session_id` bigint NOT NULL COMMENT '会话id',
    `type` tinyint NOT NULL COMMENT '消息类型：0 文本消息，1 图片消息，2 表情包，3 红包',
    `content` text CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci NOT NULL COMMENT '消息内容',
    `reply_id` bigint NULL DEFAULT NULL COMMENT '消息引用id',
    `session_type` tinyint NOT NULL COMMENT '会话类型：0 单聊，1 群聊，2 机器人会话',
    `created_time` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `updated_time` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    PRIMARY KEY (`message_id`) USING BTREE,
    INDEX `idx_session_time` (`session_id` ASC, `created_time` ASC) USING BTREE,
    INDEX `idx_sender_time` (`sender_id` ASC, `created_time` ASC) USING BTREE,
    INDEX `idx_reply_id` (`reply_id` ASC) USING BTREE
) ENGINE = InnoDB CHARACTER SET = utf8mb4 COLLATE = utf8mb4_0900_ai_ci COMMENT = '消息表' ROW_FORMAT = DYNAMIC;

CREATE TABLE `system_notification` (
    `id` bigint NOT NULL COMMENT '主键ID（雪花ID）',
    `message_id` varchar(64) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci NOT NULL COMMENT '消息ID（用于去重）',
    `receiver_id` bigint NOT NULL COMMENT '接收者用户ID',
    `type` int NOT NULL COMMENT '通知类型：101好友申请，102新会话，103群聊邀请',
    `content` json NOT NULL COMMENT '完整通知内容（JSON格式）',
    `is_read` tinyint NOT NULL DEFAULT 0 COMMENT '是否已读：0未读，1已读',
    `created_time` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `updated_time` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    PRIMARY KEY (`id`) USING BTREE,
    UNIQUE INDEX `uk_message_id` (`message_id` ASC) USING BTREE COMMENT '消息ID唯一索引，防止重复消费',
    INDEX `idx_receiver_read` (`receiver_id` ASC, `is_read` ASC) USING BTREE COMMENT '接收者+已读状态查询索引'
) ENGINE = InnoDB CHARACTER SET = utf8mb4 COLLATE = utf8mb4_0900_ai_ci COMMENT = '系统通知表' ROW_FORMAT = DYNAMIC;

CREATE TABLE `ai_role` (
    `ai_role_id` bigint NOT NULL COMMENT '角色ID',
    `user_id` bigint NOT NULL COMMENT '用户ID',
    `role_name` varchar(50) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci NOT NULL COMMENT '角色名称',
    `system_prompt` text CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci NOT NULL COMMENT '该角色的系统提示词（用户自定义）',
    `is_public` tinyint(1) NULL DEFAULT 0 COMMENT '是否公开（0=私有，1=公开，可被他人使用）',
    `created_time` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `updated_time` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    PRIMARY KEY (`ai_role_id`) USING BTREE,
    INDEX `idx_user_public` (`user_id` ASC, `is_public` ASC) USING BTREE
) ENGINE = InnoDB CHARACTER SET = utf8mb4 COLLATE = utf8mb4_0900_ai_ci COMMENT = 'AI 角色自定义表' ROW_FORMAT = DYNAMIC;

INSERT INTO `user` (`user_id`, `phone`, `email`, `password`, `nickname`, `avatar`, `gender`, `description`, `state`, `role`, `created_time`, `updated_time`, `is_delete`)
VALUES (123, NULL, '123@qq.com', '842fb0b6a791d58fe294775f5e041c28', '机器人', 'http://47.115.55.73:9000/infinite-chat/default_avatar.png', 2, NULL, 0, 0, '2025-03-26 13:30:39', '2025-03-26 13:30:39', 0);