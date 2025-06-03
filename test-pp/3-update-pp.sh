#!/bin/bash
set -e
# set -x

DEPLOYER_ADDRESS="0x8f8E2d6cF621f30e9a11309D6A56A876281Fd534"
DEPLOYER_PRIVATE_KEY="0x815405dddb0e2a99b12af775fd2929e526704e1d1aea6a0b4e74dc33e2f7fcd2"
TIME_LOCK_ADDRESS="0xEA8DCb15a6AC928C1Bf07bD677682d59d48d9eC8"
ROLLUP_MGR_ADDRESS="0x2d42E2899662EFf08b13eeb65b154b904C7a1c8a"
L1_RPC_URL="http://127.0.0.1:8545"

PWD_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(dirname "$PWD_DIR")"

schedule_data=""
execute_data=""

cast send --rpc-url "$L1_RPC_URL" --private-key "$DEPLOYER_PRIVATE_KEY" "$TIME_LOCK_ADDRESS" "$schedule_data"
sleep 70

cast send --rpc-url "$L1_RPC_URL" --private-key "$DEPLOYER_PRIVATE_KEY" "$TIME_LOCK_ADDRESS" "$execute_data"

sleep 5
cast call --rpc-url "$L1_RPC_URL" $ROLLUP_MGR_ADDRESS 'ROLLUP_MANAGER_VERSION()(string)'

make stop-old
sleep 5

make run-new