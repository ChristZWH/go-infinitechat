CREATE TABLE `system_notification`
(
    `id`           bigint                                                       NOT NULL COMMENT '主键ID（雪花ID）',
    `message_id`   varchar(64) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci NOT NULL COMMENT '消息ID（用于去重）',
    `receiver_id`  bigint                                                       NOT NULL COMMENT '接收者用户ID',
    `type`         int                                                          NOT NULL COMMENT '通知类型：101好友申请，102新会话，103群聊邀请',
    `content`      json                                                         NOT NULL COMMENT '完整通知内容（JSON格式）',
    `is_read`      tinyint                                                      NOT NULL DEFAULT 0 COMMENT '是否已读：0未读，1已读',
    `created_time` datetime                                                     NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `updated_time` datetime                                                     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    PRIMARY KEY (`id`) USING BTREE,
    UNIQUE INDEX `uk_message_id`(`message_id` ASC) USING BTREE COMMENT '消息ID唯一索引，防止重复消费',
    INDEX          `idx_receiver_read`(`receiver_id` ASC, `is_read` ASC) USING BTREE COMMENT '接收者+已读状态查询索引'
) ENGINE = InnoDB CHARACTER SET = utf8mb4 COLLATE = utf8mb4_0900_ai_ci COMMENT = '系统通知表' ROW_FORMAT = DYNAMIC;
