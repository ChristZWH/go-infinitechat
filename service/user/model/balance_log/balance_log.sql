CREATE TABLE `balance_log` (
    `balance_log_id` bigint NOT NULL COMMENT '记录 ID',
    `user_id` bigint NOT NULL COMMENT '用户 ID',
    `amount` bigint NOT NULL COMMENT '变动金额（单位：分），正数为增加，负数为减少',
    `type` tinyint NOT NULL COMMENT '变动类型：0 发送红包，1 领取红包，2 红包退回',
    `related_id` bigint NULL DEFAULT NULL COMMENT '关联 ID，如红包 ID',
    `created_time` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `updated_time` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',

    PRIMARY KEY (`balance_log_id`) USING BTREE,
    INDEX `idx_related_id`(`balance_log_id` ASC, `related_id` ASC) USING BTREE COMMENT '做账单回测/对账',
    INDEX `idx_user_id`(`user_id` ASC) USING BTREE COMMENT '查询个人日志'

) ENGINE = InnoDB CHARACTER SET = utf8mb4 COLLATE = utf8mb4_0900_ai_ci COMMENT = '余额变动记录表' ROW_FORMAT = DYNAMIC;