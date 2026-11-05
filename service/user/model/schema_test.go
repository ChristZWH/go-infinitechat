// Package model_test 是 user 模型层针对"真实表结构"的回归测试。
//
// 背景：本次审查发现 4 处手写 SQL 的列名/别名写错（updateed_time、update_time、
// `receiver_id = ?`、user_id 误用为 session_id），它们只有在真实 MySQL 上执行才会暴露。
// 这里用 EXPLAIN 级别的轻量查询做守门：语句能被 MySQL 解析、列名存在，即通过。
//
// 默认跳过；设置环境变量后执行：
//
//	$env:TEST_MYSQL_DSN = "root:password@tcp(127.0.0.1:3306)/infiniteChat?charset=utf8mb4&parseTime=True&loc=Local"
//	go test ./service/user/model/ -v
package model_test

import (
	"database/sql"
	"os"
	"testing"

	_ "github.com/go-sql-driver/mysql"
)

func testDSN(t *testing.T) string {
	t.Helper()
	dsn := os.Getenv("TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("未设置 TEST_MYSQL_DSN，跳过真实库结构回归测试")
	}
	return dsn
}

// 这些语句与 model 层手写 SQL 的列名保持一字不差。
// 全部使用不存在的负数主键，命中 0 行，不会改动任何数据。
var schemaChecks = []struct {
	name  string
	query string
}{
	{
		// 曾经写成 `updateed_time`（多一个 e）→ 1054 Unknown column
		name:  "user_balance 扣款语句的列名",
		query: "UPDATE `user_balance` SET `balance` = `balance` - 1 WHERE `user_id` = -1 AND `balance` >= 1",
	},
	{
		// 曾经写成 `update_time` → 1054
		name:  "friend 更新状态语句的列名",
		query: "UPDATE `friend` SET `status` = 1, `updated_time` = now() WHERE `user_id` = -1 AND `friend_id` = -1",
	},
	{
		// 曾经把 `receiver_id = ?` 整个包进反引号，变成不存在的列名
		name:  "apply_friend 统计总数的语句",
		query: "SELECT count(*) FROM `apply_friend` WHERE `sender_id` = -1 OR `receiver_id` = -1",
	},
	{
		name:  "apply_friend 分页查询的语句",
		query: "SELECT * FROM `apply_friend` WHERE `sender_id` = -1 OR `receiver_id` = -1 ORDER BY `updated_time` DESC LIMIT 20 OFFSET 0",
	},
	{
		// 曾经写成 `where user_id = ?` 而参数是 sessionId → 逻辑错误但不会报错，
		// 这里同时校验列名与索引列可用
		name:  "user_session 按会话查成员的语句",
		query: "SELECT `user_id` FROM `user_session` WHERE `session_id` = -1 AND `status` = 0",
	},
	{
		name:  "red_packet 条件更新过期状态",
		query: "UPDATE `red_packet` SET `status` = 2, `updated_time` = now() WHERE `red_packet_id` = -1 AND `status` = 0",
	},
}

func TestSchemaQueriesAreValid(t *testing.T) {
	db, err := sql.Open("mysql", testDSN(t))
	if err != nil {
		t.Fatalf("连接配置错误: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		t.Fatalf("无法连接 MySQL: %v", err)
	}

	for _, c := range schemaChecks {
		t.Run(c.name, func(t *testing.T) {
			// EXPLAIN 只编译不执行，能发现"列不存在 / 语法错误"，且绝不改数据
			rows, err := db.Query("EXPLAIN " + c.query)
			if err != nil {
				t.Fatalf("SQL 无法通过 MySQL 解析（列名或语法错误）: %v\nSQL: %s", err, c.query)
			}
			rows.Close()
		})
	}
}

func TestTablesExist(t *testing.T) {
	db, err := sql.Open("mysql", testDSN(t))
	if err != nil {
		t.Fatalf("连接配置错误: %v", err)
	}
	defer db.Close()

	tables := []string{"user", "user_balance", "balance_log", "friend", "apply_friend", "session", "user_session"}
	for _, table := range tables {
		t.Run(table, func(t *testing.T) {
			var name string
			err := db.QueryRow("SELECT table_name FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = ?", table).Scan(&name)
			if err == sql.ErrNoRows {
				t.Fatalf("表 %s 不存在，请先执行 IM.sql", table)
			}
			if err != nil {
				t.Fatalf("查询表元信息失败: %v", err)
			}
		})
	}
}
