-- 设备授权事务为短期、可重新发起的数据；回滚不影响既有 Grant 和凭证摘要。
DROP TABLE IF EXISTS pm_external_device_authorizations;
