DROP TABLE IF EXISTS `red_packet`;
CREATE TABLE `red_packet`
(
    `red_packet_id`           bigint                                                       NOT NULL COMMENT '红包 ID',
    `sender_id`               bigint                                                       NOT NULL COMMENT '发送者用户 ID',
    `session_id`              bigint                                                       NOT NULL COMMENT '会话 ID（单聊或群聊）',
    `session_type`            tinyint NULL COMMENT '会话类型：0 群聊，1 单聊',
    `red_packet_wrapper_text` varchar(50) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci NOT NULL DEFAULT '恭喜发财，大吉大利' COMMENT '红包封面文案',
    `red_packet_type`         tinyint                                                      NOT NULL COMMENT '红包类型：0 普通红包，1 拼手气红包',
    `total_amount`            bigint                                                       NOT NULL COMMENT '红包总金额(单位：分)',
    `total_count`             int                                                          NOT NULL COMMENT '红包总个数',
    `status`                  tinyint                                                      NOT NULL DEFAULT 0 COMMENT '状态：0 未领取完，1 已领取完，2 已过期，-1 红包不存在',
    `created_time`            datetime                                                     NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `updated_time`            datetime                                                     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    PRIMARY KEY (`red_packet_id`) USING BTREE,
    INDEX                     `idx_session_id`(`session_id` ASC) USING BTREE,
    INDEX                     `idx_sender_id`(`sender_id` ASC) USING BTREE
) ENGINE = InnoDB CHARACTER SET = utf8mb4 COLLATE = utf8mb4_0900_ai_ci COMMENT = '红包主表' ROW_FORMAT = DYNAMIC;