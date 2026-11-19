# amd64
CNB_BIN=../../script/bin/cnb-linux-amd64 \
BPS_BIN=../../script/bin/bksmartpacker-linux-amd64 \
just smart /home/debi/Desktop/cmdb-saas/release-amd64

# arm64
CNB_BIN=../../script/bin/cnb-linux-amd64 \
BPS_BIN=../../script/bin/bksmartpacker-linux-arm64 \
BKS_PLATFORM=linux/arm64 \
BUILDER_IMAGE=offline-builder-build-bk:arm64 \
just smart /home/debi/Desktop/cmdb-saas/release-arm64