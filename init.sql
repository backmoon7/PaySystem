-- 支付系统数据库初始化脚本
-- 注意：GORM 会自动创建表结构，此脚本用于添加索引和测试数据

-- 确保使用 UTF8MB4 字符集
SET NAMES utf8mb4;
SET CHARACTER SET utf8mb4;

-- 创建测试用户账户（可选）
-- INSERT INTO account (user_id, balance, frozen_amount, version, created_at, updated_at)
-- VALUES 
--   (10001, 10000, 0, 0, NOW(), NOW()),
--   (10002, 5000, 0, 0, NOW(), NOW()),
--   (10003, 20000, 0, 0, NOW(), NOW());
