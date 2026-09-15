CREATE TABLE `red_packet_receive`
(
    `red_packet_receive_id` bigint   NOT NULL COMMENT '记录 ID',
    `red_packet_id`         bigint   NOT NULL COMMENT '红包 ID',
    `receiver_id`           bigint   NOT NULL COMMENT '领取者用户 ID',
    `amount`                bigint   NOT NULL COMMENT '领取金额（单位：分）',
    `received_at`           datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '领取时间',
    `created_time`          datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `updated_time`          datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    PRIMARY KEY (`red_packet_receive_id`) USING BTREE,
    UNIQUE INDEX `idx_red_packet_receiver`(`red_packet_id` ASC, `receiver_id` ASC) USING BTREE,
    INDEX                   `idx_red_packet_id`(`red_packet_id` ASC) USING BTREE,
    INDEX                   `idx_receiver_id`(`receiver_id` ASC) USING BTREE
) ENGINE = InnoDB CHARACTER SET = utf8mb4 COLLATE = utf8mb4_0900_ai_ci COMMENT = '红包领取记录表' ROW_FORMAT = DYNAMIC;

