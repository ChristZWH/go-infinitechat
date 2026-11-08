CREATE TABLE `event_outbox` (
    `id` bigint NOT NULL AUTO_INCREMENT COMMENT '自增主键',
    `topic` varchar(64) NOT NULL COMMENT 'Kafka topic，或内部事件标记（如 __redis__pool_init__）',
    `message_key` varchar(128) NOT NULL DEFAULT '' COMMENT 'Kafka key（空=无 key）',
    `dedup_key` varchar(128) NOT NULL COMMENT '幂等去重键（INSERT IGNORE 依赖唯一索引）',
    `payload` text NOT NULL COMMENT '事件 JSON 载荷',
    `status` tinyint NOT NULL DEFAULT 0 COMMENT '0=待投递 1=已投递',
    `created_time` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `updated_time` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_dedup` (`dedup_key`),
    KEY `idx_status` (`status`)
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_0900_ai_ci COMMENT = '事务发件箱';
