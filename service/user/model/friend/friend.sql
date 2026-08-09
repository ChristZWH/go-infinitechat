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