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