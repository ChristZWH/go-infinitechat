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