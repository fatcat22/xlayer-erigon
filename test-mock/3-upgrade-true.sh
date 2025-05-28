#!/bin/bash
set -e
# set -x

DEPLOYER_ADDRESS="0x8f8E2d6cF621f30e9a11309D6A56A876281Fd534"
DEPLOYER_PRIVATE_KEY="0x815405dddb0e2a99b12af775fd2929e526704e1d1aea6a0b4e74dc33e2f7fcd2"
DEPLOYER_MNEMONIC="moment wine false celery win galaxy glide thumb tail setup choose city"
RICH_ADDRESS="0x14dC79964da2C08b23698B3D3cc7Ca32193d9955"
RICH_PRIVATE_KEY="0x4bbbf85ce3377467afe5d46f804f221813b2bb87f24d81f60f1fcdbf7cbf4356"

SEQ_ADDRESS="0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266"
SEQ_PRIVATE_KEY="0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80"
TOKEN_ADDRESS="0x5FbDB2315678afecb367f032d93F642f64180aa3"
DA_ADDRESS="0x3bFa19E4588962D1834B2e4007F150f4447Aa9fe"

PWD_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(dirname "$PWD_DIR")"

CONTRACT_JSON="artifacts/contracts/verifiers/FflonkVerifier_13.sol/FflonkVerifier_13.json"

sed_inplace() {
  if [[ "$OSTYPE" == "darwin"* ]]; then
    sed -i '' "$@"
  else
    sed -i "$@"
  fi
}

cd agglayer-contracts

BYTECODE=$(jq -r '.bytecode' "$CONTRACT_JSON")
true_contract_address=$(cast send --private-key $DEPLOYER_PRIVATE_KEY --create  "$BYTECODE" | awk '/contractAddress/ {print $2}')
echo "true_contract_address: $true_contract_address"

cast call 0x2d42E2899662EFf08b13eeb65b154b904C7a1c8a "rollupTypeCount()" 
cast call 0x2d42E2899662EFf08b13eeb65b154b904C7a1c8a "rollupTypeMap(uint32)(address,address,uint64,uint8,bool,bytes32)" 1 

echo "Creating ./tools/addRollupType/add_rollup_type.json..."
cat > ./tools/addRollupType/add_rollup_type.json << EOF
{
    "consensusContract": "PolygonValidiumEtrog",
    "polygonRollupManagerAddress": "0x2d42E2899662EFf08b13eeb65b154b904C7a1c8a",
    "polygonZkEVMBridgeAddress": "0x3a277Fa4E78cc1266F32E26c467F99A8eAEfF7c3",
    "polygonZkEVMGlobalExitRootAddress": "0xB8cedD4B9eF683f0887C44a6E4312dC7A6e2fcdB",
    "polTokenAddress": "0x5FbDB2315678afecb367f032d93F642f64180aa3",
    "verifierAddress": "$true_contract_address",
    "description": "Fork13 Validium",
    "forkID": 13,
    "rollupCompatibilityID": 0,
    "timelockDelay": 600,
    "timelockSalt": "",
    "deployerPvtKey": "",
    "maxFeePerGas":"",
    "maxPriorityFeePerGas":"",
    "multiplierGas": "",
    "genesisRoot": "0xc2c9f845f2afefd78555f7f37b6cb1c8bad8d565f81460bb809aee0d288b9d45"
}
EOF

cp ../contract/genesis.json ./tools/addRollupType/genesis.json

npx hardhat run ./tools/addRollupType/addRollupType.ts --network localhost

hex=$(cast call 0x2d42E2899662EFf08b13eeb65b154b904C7a1c8a "rollupTypeCount()")
rollupTypeCount=$((16#${hex#0x}))
echo "$rollupTypeCount"
cast call 0x2d42E2899662EFf08b13eeb65b154b904C7a1c8a "rollupTypeMap(uint32)(address,address,uint64,uint8,bool,bytes32)" $rollupTypeCount

echo "Creating ./tools/updateRollup/updateRollup.json..."
cat > ./tools/updateRollup/updateRollup.json << EOF
{
    "rollupAddress": "0xeb173087729c88a47568AF87b17C653039377BA6",
    "newRollupTypeID": $rollupTypeCount,
    "upgradeData": "0x",
    "polygonRollupManagerAddress": "0x2d42E2899662EFf08b13eeb65b154b904C7a1c8a",
    "timelockDelay": 600,
    "deployerPvtKey": "",
    "maxFeePerGas": "",
    "maxPriorityFeePerGas": "",
    "multiplierGas": ""
}
EOF

sed_inplace '97,100s/^/\/\//' tools/updateRollup/updateRollup.ts

echo "Before updateRollup.ts"
cast call 0x2d42E2899662EFf08b13eeb65b154b904C7a1c8a "rollupIDToRollupData(uint32)(address,uint64,address,uint64,bytes32,uint64,uint64,uint64,uint64,uint64,uint64,uint8)" 1 

npx hardhat run ./tools/updateRollup/updateRollup.ts --network localhost
echo "After updateRollup.ts, rollupTypeID: 1"
cast call 0x2d42E2899662EFf08b13eeb65b154b904C7a1c8a "rollupIDToRollupData(uint32)(address,uint64,address,uint64,bytes32,uint64,uint64,uint64,uint64,uint64,uint64,uint8)" 1 

cd $PWD_DIR
make run-true