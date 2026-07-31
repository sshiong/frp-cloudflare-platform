#!/bin/bash
# FRP Panel 备份脚本
# 定期备份数据库和配置文件

set -e

# 配置
BACKUP_DIR="/data/backups"
DATA_DIR="/data"
RETENTION_DAYS=30
DATE=$(date +%Y%m%d_%H%M%S)
BACKUP_NAME="frp-panel-backup-${DATE}"

# 颜色
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo -e "${GREEN}=== FRP Panel 备份开始 ===${NC}"
echo -e "时间: $(date)"

# 创建备份目录
mkdir -p "${BACKUP_DIR}"

# 创建临时目录
TEMP_DIR=$(mktemp -d)
trap "rm -rf ${TEMP_DIR}" EXIT

# 备份数据库
echo -e "${YELLOW}备份数据库...${NC}"
if [ -f "${DATA_DIR}/frp-panel.db" ]; then
    # 使用 SQLite Backup API 一致性备份
    sqlite3 "${DATA_DIR}/frp-panel.db" ".backup '${TEMP_DIR}/frp-panel.db'"
    echo -e "${GREEN}数据库备份完成${NC}"
else
    echo -e "${RED}警告: 数据库文件不存在${NC}"
fi

# 备份 WAL 和 SHM 文件
if [ -f "${DATA_DIR}/frp-panel.db-wal" ]; then
    cp "${DATA_DIR}/frp-panel.db-wal" "${TEMP_DIR}/"
fi
if [ -f "${DATA_DIR}/frp-panel.db-shm" ]; then
    cp "${DATA_DIR}/frp-panel.db-shm" "${TEMP_DIR}/"
fi

# 备份配置文件
echo -e "${YELLOW}备份配置文件...${NC}"
if [ -d "/etc/frp-panel" ]; then
    cp -r /etc/frp-panel "${TEMP_DIR}/config"
fi

# 备份证书
echo -e "${YELLOW}备份证书...${NC}"
if [ -d "${DATA_DIR}/certificates" ]; then
    cp -r "${DATA_DIR}/certificates" "${TEMP_DIR}/"
fi

# 备份密钥文件
echo -e "${YELLOW}备份密钥文件...${NC}"
if [ -d "${DATA_DIR}/secrets" ]; then
    cp -r "${DATA_DIR}/secrets" "${TEMP_DIR}/"
fi

# 备份 Router 快照
echo -e "${YELLOW}备份 Router 快照...${NC}"
if [ -d "${DATA_DIR}/router/snapshots" ]; then
    cp -r "${DATA_DIR}/router/snapshots" "${TEMP_DIR}/router-snapshots"
fi

# 创建备份清单
echo -e "${YELLOW}创建备份清单...${NC}"
cat > "${TEMP_DIR}/manifest.json" << EOF
{
  "version": "2.3.0",
  "created_at": "$(date -u +%Y-%m-%dT%H:%M:%SZ)",
  "hostname": "$(hostname)",
  "files": [
    $(cd "${TEMP_DIR}" && find . -type f -exec sh -c 'echo "{\"path\":\"$1\",\"size\":$(stat -f%z "$1" 2>/dev/null || stat -c%s "$1" 2>/dev/null),\"hash\":\"$(sha256sum "$1" | cut -d" " -f1)\"}"' _ {} \; | paste -sd,)
  ]
}
EOF

# 创建压缩包
echo -e "${YELLOW}创建压缩包...${NC}"
cd "${TEMP_DIR}"
tar -czf "${BACKUP_DIR}/${BACKUP_NAME}.tar.gz" .

# 计算校验和
echo -e "${YELLOW}计算校验和...${NC}"
sha256sum "${BACKUP_DIR}/${BACKUP_NAME}.tar.gz" > "${BACKUP_DIR}/${BACKUP_NAME}.sha256"

# 清理旧备份
echo -e "${YELLOW}清理 ${RETENTION_DAYS} 天前的备份...${NC}"
find "${BACKUP_DIR}" -name "frp-panel-backup-*.tar.gz" -mtime +${RETENTION_DAYS} -delete
find "${BACKUP_DIR}" -name "frp-panel-backup-*.sha256" -mtime +${RETENTION_DAYS} -delete

# 完成
BACKUP_SIZE=$(du -sh "${BACKUP_DIR}/${BACKUP_NAME}.tar.gz" | cut -f1)
echo -e "${GREEN}=== 备份完成 ===${NC}"
echo -e "备份文件: ${BACKUP_DIR}/${BACKUP_NAME}.tar.gz"
echo -e "文件大小: ${BACKUP_SIZE}"
echo -e "校验和: ${BACKUP_DIR}/${BACKUP_NAME}.sha256"

# 记录审计日志
echo "$(date -u +%Y-%m-%dT%H:%M:%SZ) [INFO] Backup completed: ${BACKUP_NAME}.tar.gz (${BACKUP_SIZE})" >> /var/log/frp-panel/backup.log
