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

sed_inplace() {
  if [[ "$OSTYPE" == "darwin"* ]]; then
    sed -i '' "$@"
  else
    sed -i "$@"
  fi
}

if [ ! -d "./xlayer-contracts" ]; then
  echo "No contract repository found. Please clone the repository first."
  exit 1
fi

cd "./xlayer-contracts"

git stash
git checkout upstream/v9.0.0-rc.3-pp; git pull
git stash apply
rm -rf artifacts cache node_modules
npm i

cat upgrade/upgradePessimistic/upgrade_parameters.json.example |
    jq --arg rum $ROLLUP_MGR_ADDRESS \
       --arg sk $DEPLOYER_PRIVATE_KEY \
       --arg tld 60 '.rollupManagerAddress = $rum | .timelockDelay = $tld | .deployerPvtKey = $sk' > upgrade/upgradePessimistic/upgrade_parameters.json

hardhat_output=$(npx hardhat run ./upgrade/upgradePessimistic/upgradePessimistic.ts --network localhost)
echo "hardhat_output: $hardhat_output"

schedule_data=$(echo "$hardhat_output" | awk -F"'" '/scheduleData:/ {print $2}')
execute_data=$(echo "$hardhat_output" | awk -F"'" '/executeData:/ {print $2}')
echo "schedule_data: $schedule_data"
echo "execute_data: $execute_data"


cast send --rpc-url "$L1_RPC_URL" --private-key "$DEPLOYER_PRIVATE_KEY" "$TIME_LOCK_ADDRESS" "$schedule_data"
sleep 70

cast send --rpc-url "$L1_RPC_URL" --private-key "$DEPLOYER_PRIVATE_KEY" "$TIME_LOCK_ADDRESS" "$execute_data"

sleep 5
cast call --rpc-url "$L1_RPC_URL" $ROLLUP_MGR_ADDRESS 'ROLLUP_MANAGER_VERSION()(string)'
