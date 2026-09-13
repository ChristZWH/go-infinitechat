CREATE TABLE `message`
(
    `message_id`   bigint   NOT NULL COMMENT '消息 id',
    `sender_id`    bigint   NOT NULL COMMENT '发送者 id',
    `session_id`   bigint   NOT NULL COMMENT '会话 id',
    `type`         tinyint  NOT NULL COMMENT '消息类型: 0 文本消息，1 图片消息，2 表情包，3 红包',
    `content`      text CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci NOT NULL COMMENT '消息内容',
    `reply_id`     bigint NULL DEFAULT NULL COMMENT '消息引用 id',
    `session_type` tinyint  NOT NULL COMMENT '会话类型: 0 单聊，1 群聊，2机器人会话',
    `created_time` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `updated_time` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    PRIMARY KEY (`message_id`) USING BTREE,
    INDEX          `idx_session_time`(`session_id` ASC, `created_time` ASC) USING BTREE,
    INDEX          `idx_sender_time`(`sender_id` ASC, `created_time` ASC) USING BTREE,
    INDEX          `idx_reply_id`(`reply_id` ASC) USING BTREE
) ENGINE = InnoDB CHARACTER SET = utf8mb4 COLLATE = utf8mb4_0900_ai_ci COMMENT = '消息表' ROW_FORMAT = DYNAMIC;
