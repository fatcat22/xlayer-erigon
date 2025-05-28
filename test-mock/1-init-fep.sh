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

make stop

sed_inplace() {
  if [[ "$OSTYPE" == "darwin"* ]]; then
    sed -i '' "$@"
  else
    sed -i "$@"
  fi
}

echo "Cleaning all docker containers..."
docker stop $(docker ps -aq) || true
docker rm $(docker ps -aq) || true

echo "Starting xlayer-mock-l1-network..."
docker-compose up -d xlayer-mock-l1-network
sleep 5

echo "Sending funds to deployer..."
cast send -f $RICH_ADDRESS --private-key $RICH_PRIVATE_KEY --value 3ether --legacy $DEPLOYER_ADDRESS

if [ ! -d "./agglayer-contracts" ]; then
  echo "Cloning contract repository..."
  git clone -b v8.1.0-rc.1-fork.13 https://github.com/agglayer/agglayer-contracts.git
fi

cd ./agglayer-contracts
echo "Cleaning and resting contract repository..."
rm -rf *; git reset --hard; git checkout v8.1.0-rc.1-fork.13

echo "Creating .env file..."
cat > .env << EOF
MNEMONIC="$DEPLOYER_MNEMONIC"
INFURA_PROJECT_ID="000"
ETHERSCAN_API_KEY="000"
EOF

cd deployment/v2

echo "Creating create_rollup_parameters.json..."
cat > create_rollup_parameters.json << EOF
{
    "realVerifier": true,
    "trustedSequencerURL": "http://xlayer-rpc:8545",
    "networkName": "zkevm",
    "description":"0.0.1",
    "trustedSequencer":"0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266",
    "chainID": 195,
    "adminZkEVM":"0x8f8E2d6cF621f30e9a11309D6A56A876281Fd534",
    "forkID": 13,
    "consensusContract": "PolygonValidiumEtrog",
    "dataAvailabilityProtocol": "PolygonDataCommittee",
    "gasTokenAddress":"0x5FbDB2315678afecb367f032d93F642f64180aa3",
    "deployerPvtKey": "",
    "maxFeePerGas":"",
    "maxPriorityFeePerGas":"",
    "multiplierGas": ""
}
EOF

echo "Creating deploy_parameters.json..."
cat > deploy_parameters.json << EOF
{
    "test": true,
    "timelockAdminAddress": "0x8f8E2d6cF621f30e9a11309D6A56A876281Fd534",
    "minDelayTimelock": 600,
    "salt": "0x0000000000000000000000000000000000000000000000000000000000000000",
    "initialZkEVMDeployerOwner": "0x8f8E2d6cF621f30e9a11309D6A56A876281Fd534",
    "admin": "0x8f8E2d6cF621f30e9a11309D6A56A876281Fd534",
    "trustedAggregator": "0x70997970C51812dc3A010C7d01b50e0d17dc79C8",
    "trustedAggregatorTimeout": 604799,
    "pendingStateTimeout": 604799,
    "emergencyCouncilAddress": "0x8f8E2d6cF621f30e9a11309D6A56A876281Fd534",
    "polTokenAddress":"0x5FbDB2315678afecb367f032d93F642f64180aa3",
    "zkEVMDeployerAddress":"0x8f8E2d6cF621f30e9a11309D6A56A876281Fd534",
    "deployerPvtKey": "",
    "maxFeePerGas":"",
    "maxPriorityFeePerGas":"",
    "multiplierGas": ""
}
EOF

echo "Compiling contracts..."
cd ../../
npm i
npm run deploy:v2:localhost

cd "$ROOT_DIR"
ROLLUP_OUTPUT_PATH="./test-mock/agglayer-contracts/deployment/v2/create_rollup_output.json"

rm -rf ./test-mock/contract/*
cp -rf $ROLLUP_OUTPUT_PATH ./test-mock/contract/create_rollup_output.json
cp -rf ./test-mock/agglayer-contracts/deployment/v2/create_rollup_parameters.json ./test-mock/contract/
cp -rf ./test-mock/agglayer-contracts/deployment/v2/deploy_parameters.json ./test-mock/contract/
cp -rf ./test-mock/agglayer-contracts/deployment/v2/deploy_output.json ./test-mock/contract/
cp -rf ./test-mock/agglayer-contracts/deployment/v2/genesis.json ./test-mock/contract/
ROLLUP_OUTPUT_PATH="./test-mock/contract/create_rollup_output.json"
DEPLOY_OUTPUT_PATH="./test-mock/contract/deploy_output.json"

echo "Transferring ERC20 token to Sequencer..."
cast send --legacy --from $SEQ_ADDRESS --private-key $SEQ_PRIVATE_KEY $TOKEN_ADDRESS "transfer(address,uint256)" $SEQ_ADDRESS 1000

cast call $DA_ADDRESS 'function getAmountOfMembers() public returns (uint256)'
cast send --legacy --from $DEPLOYER_ADDRESS --private-key $DEPLOYER_PRIVATE_KEY  $DA_ADDRESS 'function setupCommittee(uint256 _requiredAmountOfSignatures, string[] urls, bytes addrsBytes) returns()' 1 [http://xlayer-da:8444] $SEQ_ADDRESS
cast call $DA_ADDRESS 'function getAmountOfMembers() public returns (uint256)'

echo "Setting Trusted Sequencer URL..."
POE_ADDRESS=$(cat $ROLLUP_OUTPUT_PATH | grep -o '"rollupAddress": "[^"]*"' | cut -d'"' -f4)
BRIDGE_ADDRESS=$(cat $DEPLOY_OUTPUT_PATH | grep -o '"polygonZkEVMBridgeAddress": "[^"]*"' | cut -d'"' -f4)
GENESIS_VALUE=$(cat $ROLLUP_OUTPUT_PATH | grep -o '"genesis": "[^"]*"' | cut -d'"' -f4)
TIMESTAMP_VALUE=$(cat $ROLLUP_OUTPUT_PATH | grep -o '"timestamp": [0-9]*' | cut -d' ' -f2)
L1_FIRST_BLOCK=$(cat $DEPLOY_OUTPUT_PATH | grep -o '"upgradeToULxLyBlockNumber": [0-9]*' | cut -d' ' -f2)
L1_SECOND_BLOCK=$(cat $ROLLUP_OUTPUT_PATH | grep -o '"createRollupBlockNumber": [0-9]*' | cut -d' ' -f2)
ROLLUP_MANAGER_ADDRESS=$(grep -o '"polygonRollupManagerAddress": "[^"]*"' "$DEPLOY_OUTPUT_PATH" | cut -d'"' -f4)
GLOBAL_EXIT_ROOT_ADDRESS=$(grep -o '"polygonZkEVMGlobalExitRootAddress": "[^"]*"' "$DEPLOY_OUTPUT_PATH" | cut -d'"' -f4)
echo "Poe address from JSON: $POE_ADDRESS"
echo "Bridge address from JSON: $BRIDGE_ADDRESS"
echo "Genesis value from JSON: $GENESIS_VALUE"
echo "Timestamp value from JSON: $TIMESTAMP_VALUE"
echo "L1FirstBlock value from JSON: $L1_FIRST_BLOCK"
echo "L1SecondBlock value from JSON: $L1_SECOND_BLOCK"
echo "RollupManagerAddress value from JSON: $ROLLUP_MANAGER_ADDRESS"
echo "GlobalExitRootAddress value from JSON: $GLOBAL_EXIT_ROOT_ADDRESS"

cast send --legacy --from $DEPLOYER_ADDRESS --private-key $DEPLOYER_PRIVATE_KEY $POE_ADDRESS "setTrustedSequencerURL(string)" "http://xlayer-rpc:8545"

cast send --legacy --from $DEPLOYER_ADDRESS --private-key $DEPLOYER_PRIVATE_KEY $BRIDGE_ADDRESS 'function bridgeAsset(uint32 destinationNetwork, address destinationAddress, uint256 amount, address token, bool forceUpdateGlobalExitRoot, bytes permitData) returns()' 7 0x0000000000000000000000000000000000000000 0 0x0000000000000000000000000000000000000000 true 0x

echo "Generating configuration files..."
go install ./cmd/hack/allocs
which allocs
allocs ./test-mock/agglayer-contracts/deployment/v2/genesis.json
mv allocs.json ./test-mock/config/dynamic-mynetwork-allocs.json

cat > ./test-mock/config/dynamic-mynetwork-conf.json << EOF
{
  "root": "$GENESIS_VALUE",
  "timestamp": $TIMESTAMP_VALUE,
  "gasLimit": 0,
  "difficulty": 0
}
EOF
echo "dynamic-mynetwork-conf.json file updated"

echo "Updating test.erigon.seq.config.yaml file..."
CONFIG_FILE="./test-mock/config/test.erigon.seq.config.yaml"
sed_inplace "s|zkevm.address-zkevm: \"[^\"]*\"|zkevm.address-zkevm: \"$POE_ADDRESS\"|g" $CONFIG_FILE
sed_inplace "s|zkevm.address-rollup: \"[^\"]*\"|zkevm.address-rollup: \"$ROLLUP_MANAGER_ADDRESS\"|g" $CONFIG_FILE
sed_inplace "s|zkevm.address-ger-manager: \"[^\"]*\"|zkevm.address-ger-manager: \"$GLOBAL_EXIT_ROOT_ADDRESS\"|g" $CONFIG_FILE
sed_inplace "s|zkevm.l1-first-block: [0-9]*|zkevm.l1-first-block: $L1_FIRST_BLOCK|g" $CONFIG_FILE

mkdir -p "$PWD_DIR/config"

GENESIS_CONFIG_FILE="./test-mock/config/test.genesis.config.json"
sed_inplace "s|\"genesisBlockNumber\": [0-9]*|\"genesisBlockNumber\": $L1_FIRST_BLOCK|" "$GENESIS_CONFIG_FILE"
sed_inplace "s|\"rollupCreationBlockNumber\": [0-9]*|\"rollupCreationBlockNumber\": $L1_SECOND_BLOCK|" "$GENESIS_CONFIG_FILE"
sed_inplace "s|\"rollupManagerCreationBlockNumber\": [0-9]*|\"rollupManagerCreationBlockNumber\": $L1_FIRST_BLOCK|" "$GENESIS_CONFIG_FILE"
echo "Initialization script completed!"

cd "$PWD_DIR"

if [ ! -d "./prover-data" ]; then
  echo "downlad prover data..."
  mkdir -p prover-data; cd prover-data;
  wget https://storage.googleapis.com/zkevm/zkproverc/v9.0.0-rc.3-fork.13.tgz
  tar -xzf v9.0.0-rc.3-fork.13.tgz
fi

cd "$PWD_DIR"
make run