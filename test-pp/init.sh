#!/bin/bash
# Exit on error
set -e
# set -x

# Define base working directory
BASE_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(dirname "$BASE_DIR")"
echo "Base working directory: $BASE_DIR"
echo "Parent directory: $ROOT_DIR"
# Define configurable parameters
DOCKER_IMAGE_NAME=${DOCKER_IMAGE_NAME:-"zjg555543/geth"}
DOCKER_IMAGE_TAG=${DOCKER_IMAGE_TAG:-"pp-v5"}

echo "Starting initialization script..."

make stop

# Clean docker
echo "Cleaning all docker containers..."
docker stop $(docker ps -aq) || true
docker rm $(docker ps -aq) || true
docker ps -a

# # Start mock l1
echo "Starting zkevm-mock-l1-network..."
docker-compose up -d zkevm-mock-l1-network

# Wait a few seconds to ensure the network is up
echo "Waiting for network to start..."
sleep 5

# ## Prepare contract deployer
# echo "Preparing contract deployer..."
DEPLOYER_ADDRESS="0x8f8E2d6cF621f30e9a11309D6A56A876281Fd534"
DEPLOYER_PRIVATE_KEY="0x815405dddb0e2a99b12af775fd2929e526704e1d1aea6a0b4e74dc33e2f7fcd2"
DEPLOYER_MNEMONIC="moment wine false celery win galaxy glide thumb tail setup choose city"

RICH_ADDRESS="0x14dC79964da2C08b23698B3D3cc7Ca32193d9955"
RICH_PRIVATE_KEY="0x4bbbf85ce3377467afe5d46f804f221813b2bb87f24d81f60f1fcdbf7cbf4356"

### Check balance
echo "Checking rich address balance..."
cast rpc eth_getBalance $RICH_ADDRESS latest

### Send funds to deployer
echo "Sending funds to deployer..."
cast send -f $RICH_ADDRESS --private-key $RICH_PRIVATE_KEY --value 3ether --legacy $DEPLOYER_ADDRESS

# Reset contract environment
if [ ! -d "./agglayer-contracts" ]; then
  echo "Cloning contract repository..."
  git clone -b v10.0.0-rc.6 https://github.com/agglayer/agglayer-contracts.git
fi

cd ./agglayer-contracts
echo "Cleaning and resting contract repository..."
rm -rf *; git reset --hard

# Create .env file
echo "Creating .env file..."
cat > .env << EOF
MNEMONIC="$DEPLOYER_MNEMONIC"
INFURA_PROJECT_ID="6d3d0adfe7c74dcb87642b37f1477aec"
ETHERSCAN_API_KEY="F948SAS2HRYQ8V8Y315RWMT1E21M3K3PTK"
EOF

cd deployment/v2

# Create create_rollup_parameters.json
echo "Creating create_rollup_parameters.json..."
cat > create_rollup_parameters.json << EOF
{
    "adminZkEVM": "$DEPLOYER_ADDRESS",
    "chainID": 195,
    "consensusContract": "PolygonPessimisticConsensus",
    "dataAvailabilityProtocol": "PolygonDataCommittee",
    "deployerPvtKey": "",
    "description": "description",
    "forkID": 13,
    "gasTokenAddress":"0x5FbDB2315678afecb367f032d93F642f64180aa3",
    "maxFeePerGas": "",
    "maxPriorityFeePerGas": "",
    "multiplierGas": "",
    "networkName": "zkevm",
    "realVerifier": false,
    "trustedSequencer": "0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266",
    "trustedSequencerURL": "http://xlayer-seq:8545",
    "trustedAggregator":"0x70997970C51812dc3A010C7d01b50e0d17dc79C8",
    "programVKey": "0x00d6e4bdab9cac75a50d58262bb4e60b3107a6b61131ccdff649576c624b6fb7"
}
EOF

# Create deploy_parameters.json
echo "Creating deploy_parameters.json..."
cat > deploy_parameters.json << EOF
{
    "admin": "$DEPLOYER_ADDRESS",
    "deployerPvtKey": "",
    "emergencyCouncilAddress": "$DEPLOYER_ADDRESS",
    "initialZkEVMDeployerOwner": "$DEPLOYER_ADDRESS",
    "maxFeePerGas": "",
    "maxPriorityFeePerGas": "",
    "minDelayTimelock": 60,
    "multiplierGas": "",
    "description": "description",
    "pendingStateTimeout": 604799,
    "polTokenAddress": "0x5FbDB2315678afecb367f032d93F642f64180aa3",
    "salt": "0x0000000000000000000000000000000000000000000000000000000000000001",
    "timelockAdminAddress": "$DEPLOYER_ADDRESS",
    "trustedSequencer": "0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266",
    "trustedSequencerURL": "http://xlayer-seq:8545",
    "trustedAggregator": "0x70997970C51812dc3A010C7d01b50e0d17dc79C8",
    "trustedAggregatorTimeout": 604799,
    "forkID": 13,
    "test": true,
    "ppVKey": "0x00d6e4bdab9cac75a50d58262bb4e60b3107a6b61131ccdff649576c624b6fb7",
    "ppVKeySelector": "0x00000001",
    "realVerifier": false,
    "defaultAdminAddress": "$DEPLOYER_ADDRESS",
    "aggchainDefaultVKeyRoleAddress": "$DEPLOYER_ADDRESS",
    "addRouteRoleAddress": "$DEPLOYER_ADDRESS",
    "freezeRouteRoleAddress": "$DEPLOYER_ADDRESS",
    "zkEVMDeployerAddress": "$DEPLOYER_ADDRESS"
}
EOF

# Compile contracts
echo "Compiling contracts..."
cd ../../
npm i
npm run deploy:v2:localhost

# View genesis file
echo "Viewing genesis file..."
cat deployment/v2/genesis.json
cat deployment/v2/deploy_output.json 
# find latest create_rollup_output file
LATEST_ROLLUP_OUTPUT=$(find deployment/v2 -name "create_rollup_output_*.json" | sort -r | head -n 1)
echo "Using rollup output file: $LATEST_ROLLUP_OUTPUT"
cat $LATEST_ROLLUP_OUTPUT

# Transfer ERC20 token to Sequencer
echo "Transferring ERC20 token to Sequencer..."
SEQ_ADDRESS="0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266"
SEQ_PRIVATE_KEY="0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80"
TOKEN_ADDRESS="0x5FbDB2315678afecb367f032d93F642f64180aa3"
cast send --legacy --from $SEQ_ADDRESS --private-key $SEQ_PRIVATE_KEY $TOKEN_ADDRESS "transfer(address,uint256)" $SEQ_ADDRESS 1000

cd "$ROOT_DIR"
# Set Trusted RPC
echo "Setting Trusted Sequencer URL..."
echo "Current directory: $(pwd)"
# Read rollupAddress from the latest create_rollup_output_*.json file
ROLLUP_OUTPUT_PATH=$(find ./test-pp/agglayer-contracts/deployment/v2 -name "create_rollup_output_*.json" | sort -r | head -n 1)
echo "Using output file: $ROLLUP_OUTPUT_PATH"
POE_ADDRESS=$(cat $ROLLUP_OUTPUT_PATH | grep -o '"rollupAddress": "[^"]*"' | cut -d'"' -f4)
echo "Using POE address from JSON: $POE_ADDRESS"
cast send --legacy --from $DEPLOYER_ADDRESS --private-key $DEPLOYER_PRIVATE_KEY $POE_ADDRESS "setTrustedSequencerURL(string)" "http://xlayer-rpc:8545"

# Cross-chain activation
echo "Activating cross-chain functionality..."
# Read BRIDGE_ADDRESS from deploy_output.json
DEPLOY_OUTPUT_PATH="./test-pp/agglayer-contracts/deployment/v2/deploy_output.json"
BRIDGE_ADDRESS=$(cat $DEPLOY_OUTPUT_PATH | grep -o '"polygonZkEVMBridgeAddress": "[^"]*"' | cut -d'"' -f4)
echo "Using Bridge address from JSON: $BRIDGE_ADDRESS"
cast send --legacy --from $DEPLOYER_ADDRESS --private-key $DEPLOYER_PRIVATE_KEY $BRIDGE_ADDRESS 'function bridgeAsset(uint32 destinationNetwork, address destinationAddress, uint256 amount, address token, bool forceUpdateGlobalExitRoot, bytes permitData) returns()' 7 0x0000000000000000000000000000000000000000 0 0x0000000000000000000000000000000000000000 true 0x

echo "Viewing running containers..."
docker ps

cast send -f 0x8f8E2d6cF621f30e9a11309D6A56A876281Fd534  --private-key 0x815405dddb0e2a99b12af775fd2929e526704e1d1aea6a0b4e74dc33e2f7fcd2 --value 0.01ether 0xf39fd6e51aad88f6f4ce6ab8827279cfffb92266 --legacy --rpc-url http://127.0.0.1:8123

# Get container ID
CONTAINER_ID=$(docker ps | grep zkevm-mock-l1-network | awk '{print $1}')
if [ -n "$CONTAINER_ID" ]; then
  echo "Entering container $CONTAINER_ID..."
  docker exec -it $CONTAINER_ID /bin/sh -c "ps -ef | grep geth; kill -15 \$(ps -ef | grep geth | grep -v grep | awk '{print \$1}')"
fi

echo "------------------------------------------------------------"
echo "Image creation ends here, you can refer to the following to commit the image"
# echo "docker ps -a"
# echo "docker commit $CONTAINER_ID ${DOCKER_IMAGE_NAME}:${DOCKER_IMAGE_TAG}"
# echo "docker push ${DOCKER_IMAGE_NAME}:${DOCKER_IMAGE_TAG}"

echo "Generating configuration files..."
go install ./cmd/hack/allocs
which allocs
allocs ./test-pp/agglayer-contracts/deployment/v2/genesis.json
mv allocs.json ./test-pp/config/dynamic-mynetwork-allocs.json

# Update dynamic-mynetwork-conf.json file
echo "Updating dynamic-mynetwork-conf.json file..."
# Read genesis and timestamp from the latest create_rollup_output_*.json file
ROLLUP_OUTPUT_PATH=$(find ./test-pp/agglayer-contracts/deployment/v2 -name "create_rollup_output_*.json" | sort -r | head -n 1)
echo "Using output file: $ROLLUP_OUTPUT_PATH"
GENESIS_VALUE=$(cat $ROLLUP_OUTPUT_PATH | grep -o '"genesis": "[^"]*"' | cut -d'"' -f4)
TIMESTAMP_VALUE=$(cat $ROLLUP_OUTPUT_PATH | grep -o '"timestamp": [0-9]*' | cut -d' ' -f2)
echo "Genesis value from JSON: $GENESIS_VALUE"
echo "Timestamp value from JSON: $TIMESTAMP_VALUE"

# Update dynamic-mynetwork-conf.json file
cat > ./test-pp/config/dynamic-mynetwork-conf.json << EOF
{
  "root": "$GENESIS_VALUE",
  "timestamp": $TIMESTAMP_VALUE,
  "gasLimit": 0,
  "difficulty": 0
}
EOF
echo "dynamic-mynetwork-conf.json file updated"

echo "Copy config files to ./test-pp/config"
cp ./test-pp/agglayer-contracts/deployment/v2/genesis.json ./test-pp/config/genesis.json

# Replace test.erigon.seq.config.yml
echo "Updating test.erigon.seq.config.yaml file..."

# Get addresses from deployment files
ROLLUP_OUTPUT_PATH=$(find ./test-pp/agglayer-contracts/deployment/v2 -name "create_rollup_output_*.json" | sort -r | head -n 1)
echo "Using output file: $ROLLUP_OUTPUT_PATH"
DEPLOY_OUTPUT_PATH="./test-pp/agglayer-contracts/deployment/v2/deploy_output.json"

# Check if files exist
if [ ! -f "$ROLLUP_OUTPUT_PATH" ]; then
  echo "Error: File not found $ROLLUP_OUTPUT_PATH"
  exit 1
fi

if [ ! -f "$DEPLOY_OUTPUT_PATH" ]; then
  echo "Error: File not found $DEPLOY_OUTPUT_PATH"
  exit 1
fi

# Read new address values
ROLLUP_ADDRESS=$(cat $DEPLOY_OUTPUT_PATH | grep -o '"polygonRollupManagerAddress": "[^"]*"' | cut -d'"' -f4)
if [ -z "$ROLLUP_ADDRESS" ]; then
  echo "Error: polygonRollupManagerAddress field not found in $DEPLOY_OUTPUT_PATH"
  exit 1
fi

ZKEVM_ADDRESS=$(cat $ROLLUP_OUTPUT_PATH | grep -o '"rollupAddress": "[^"]*"' | cut -d'"' -f4)
if [ -z "$ZKEVM_ADDRESS" ]; then
  echo "Error: rollupAddress field not found in $ROLLUP_OUTPUT_PATH"
  exit 1
fi

GER_MANAGER_ADDRESS=$(cat $DEPLOY_OUTPUT_PATH | grep -o '"polygonZkEVMGlobalExitRootAddress": "[^"]*"' | cut -d'"' -f4)
if [ -z "$GER_MANAGER_ADDRESS" ]; then
  echo "Error: polygonZkEVMGlobalExitRootAddress field not found in $DEPLOY_OUTPUT_PATH"
  exit 1
fi

L1_FIRST_BLOCK=$(cat $DEPLOY_OUTPUT_PATH | grep -o '"upgradeToULxLyBlockNumber": [0-9]*' | cut -d' ' -f2)
if [ -z "$L1_FIRST_BLOCK" ]; then
  echo "Error: upgradeToULxLyBlockNumber field not found in $DEPLOY_OUTPUT_PATH"
  exit 1
fi

echo "ZKEVM address: $ZKEVM_ADDRESS"
echo "ROLLUP address: $ROLLUP_ADDRESS"
echo "GER_MANAGER address: $GER_MANAGER_ADDRESS"
echo "L1_FIRST_BLOCK: $L1_FIRST_BLOCK"

# Use sed to replace values in config file
CONFIG_FILE="./test-pp/config/test.erigon.seq.config.yaml"
sed -i '' "s|zkevm.address-zkevm: \"[^\"]*\"|zkevm.address-zkevm: \"$ZKEVM_ADDRESS\"|g" $CONFIG_FILE
sed -i '' "s|zkevm.address-rollup: \"[^\"]*\"|zkevm.address-rollup: \"$ROLLUP_ADDRESS\"|g" $CONFIG_FILE
sed -i '' "s|zkevm.address-ger-manager: \"[^\"]*\"|zkevm.address-ger-manager: \"$GER_MANAGER_ADDRESS\"|g" $CONFIG_FILE
sed -i '' "s|zkevm.l1-first-block: [0-9]*|zkevm.l1-first-block: $L1_FIRST_BLOCK|g" $CONFIG_FILE

# Export firstBatchData to ./test-pp/config/first-batch-config.json
mkdir -p "$BASE_DIR/config"
ROLLUP_OUTPUT_PATH=$(find ./test-pp/agglayer-contracts/deployment/v2 -name "create_rollup_output_*.json" | sort -r | head -n 1)
jq '.firstBatchData' "$ROLLUP_OUTPUT_PATH" > "$BASE_DIR/config/first-batch-config.json"
echo "Successfully exported firstBatchData to $BASE_DIR/config/first-batch-config.json"
echo "test.erigon.seq.config.yaml file updated"

cd $BASE_DIR
# Reset contract environment
if [ ! -d "./cdk" ]; then
  echo "Cloning contract repository..."
  git clone -b v0.5.4-rc1 https://github.com/0xPolygon/cdk.git
fi

cd ./cdk
make build-docker
cd -

if [ ! -d "./agglayer" ]; then
  echo "Cloning contract repository..."
  git clone -b v0.3.0-rc.16 https://github.com/agglayer/agglayer.git
fi

cd ./agglayer
docker build -t agglayer .
cd $ROOT_DIR
DEPLOY_OUTPUT_PATH="./test-pp/agglayer-contracts/deployment/v2/deploy_output.json"

echo "Updating polygonBridgeAddr parameter in cdk-node-config.toml..."
# Find the latest deploy_output.json file
if [ -f "$DEPLOY_OUTPUT_PATH" ]; then
  # Extract polygonZkEVMBridgeAddress value from deploy_output.json
  BRIDGE_ADDRESS=$(grep -o '"polygonZkEVMBridgeAddress": "[^"]*"' "$DEPLOY_OUTPUT_PATH" | cut -d'"' -f4)
  echo "Bridge address obtained from deploy_output.json: $BRIDGE_ADDRESS"
  
  # Check if the address was successfully obtained
  if [ -n "$BRIDGE_ADDRESS" ]; then
    # Update polygonBridgeAddr parameter in cdk-node-config.toml
    CONFIG_FILE="./test-pp/config/cdk-node-config.toml"
    if [ -f "$CONFIG_FILE" ]; then
      # Use sed to replace polygonBridgeAddr value in the config file
      sed -i '' "s|polygonBridgeAddr = \"[^\"]*\"|polygonBridgeAddr = \"$BRIDGE_ADDRESS\"|" "$CONFIG_FILE"
      echo "Successfully updated polygonBridgeAddr in cdk-node-config.toml to: $BRIDGE_ADDRESS"
    else
      echo "Error: Config file $CONFIG_FILE does not exist"
    fi
  else
    echo "Error: Unable to extract Bridge address from deploy_output.json"
  fi
else
  echo "Error: deploy_output.json file does not exist: $DEPLOY_OUTPUT_PATH"
fi

# Replace block number parameters in cdk-node-config.toml
echo "Updating block number parameters in cdk-node-config.toml..."
if [ -f "$DEPLOY_OUTPUT_PATH" ]; then
  # Extract upgradeToULxLyBlockNumber value from deploy_output.json
  BLOCK_NUMBER=$(grep -o '"upgradeToULxLyBlockNumber": [0-9]*' "$DEPLOY_OUTPUT_PATH" | cut -d' ' -f2)
  echo "Block number obtained from deploy_output.json: $BLOCK_NUMBER"
  
  # Check if the block number was successfully obtained
  if [ -n "$BLOCK_NUMBER" ]; then
    # Update the three block number parameters in cdk-node-config.toml
    CONFIG_FILE="./test-pp/config/cdk-node-config.toml"
    if [ -f "$CONFIG_FILE" ]; then
      # Use sed to replace block number values in the config file
      sed -i '' "s|rollupCreationBlockNumber = \"[^\"]*\"|rollupCreationBlockNumber = \"$BLOCK_NUMBER\"|" "$CONFIG_FILE"
      sed -i '' "s|rollupManagerCreationBlockNumber = \"[^\"]*\"|rollupManagerCreationBlockNumber = \"$BLOCK_NUMBER\"|" "$CONFIG_FILE"
      sed -i '' "s|genesisBlockNumber = \"[^\"]*\"|genesisBlockNumber = \"$BLOCK_NUMBER\"|" "$CONFIG_FILE"
      echo "Successfully updated block number parameters in cdk-node-config.toml to: $BLOCK_NUMBER"
    else
      echo "Error: Config file $CONFIG_FILE does not exist"
    fi
  else
    echo "Error: Unable to extract block number from deploy_output.json"
  fi
else
  echo "Error: deploy_output.json file does not exist: $DEPLOY_OUTPUT_PATH"
fi

# Read from DEPLOY_OUTPUT_PATH
echo "Updating contract address parameters in cdk-node-config.toml..."

# Check if files exist
if [ ! -f "$DEPLOY_OUTPUT_PATH" ]; then
  echo "Error: deploy_output.json file does not exist: $DEPLOY_OUTPUT_PATH"
  exit 1
fi

if [ ! -f "$ROLLUP_OUTPUT_PATH" ]; then
  echo "Error: create_rollup_output file does not exist: $ROLLUP_OUTPUT_PATH"
  exit 1
fi

# Read variables from deploy_output.json
ROLLUP_MANAGER_ADDRESS=$(grep -o '"polygonRollupManagerAddress": "[^"]*"' "$DEPLOY_OUTPUT_PATH" | cut -d'"' -f4)
BRIDGE_ADDRESS=$(grep -o '"polygonZkEVMBridgeAddress": "[^"]*"' "$DEPLOY_OUTPUT_PATH" | cut -d'"' -f4)
GLOBAL_EXIT_ROOT_ADDRESS=$(grep -o '"polygonZkEVMGlobalExitRootAddress": "[^"]*"' "$DEPLOY_OUTPUT_PATH" | cut -d'"' -f4)

# Read rollupAddress from create_rollup_output file
ROLLUP_ADDRESS=$(grep -o '"rollupAddress": "[^"]*"' "$ROLLUP_OUTPUT_PATH" | cut -d'"' -f4)

# Check if all addresses were successfully obtained
if [ -z "$ROLLUP_MANAGER_ADDRESS" ]; then
  echo "Error: Unable to extract polygonRollupManagerAddress from deploy_output.json"
  exit 1
fi

if [ -z "$BRIDGE_ADDRESS" ]; then
  echo "Error: Unable to extract polygonZkEVMBridgeAddress from deploy_output.json"
  exit 1
fi

if [ -z "$GLOBAL_EXIT_ROOT_ADDRESS" ]; then
  echo "Error: Unable to extract polygonZkEVMGlobalExitRootAddress from deploy_output.json"
  exit 1
fi

if [ -z "$ROLLUP_ADDRESS" ]; then
  echo "Error: Unable to extract rollupAddress from create_rollup_output file"
  exit 1
fi

# Output the obtained addresses
echo "Addresses obtained from JSON files:"
echo "polygonRollupManagerAddress: $ROLLUP_MANAGER_ADDRESS"
echo "polygonZkEVMBridgeAddress: $BRIDGE_ADDRESS"
echo "polygonZkEVMGlobalExitRootAddress: $GLOBAL_EXIT_ROOT_ADDRESS"
echo "rollupAddress: $ROLLUP_ADDRESS"

# Update address parameters in cdk-node-config.toml
CONFIG_FILE="./test-pp/config/cdk-node-config.toml"
if [ -f "$CONFIG_FILE" ]; then
  # Use sed to replace address values in the config file
  sed -i '' "s|polygonRollupManagerAddress = \"[^\"]*\"|polygonRollupManagerAddress = \"$ROLLUP_MANAGER_ADDRESS\"|" "$CONFIG_FILE"
  sed -i '' "s|polygonZkEVMBridgeAddress = \"[^\"]*\"|polygonZkEVMBridgeAddress = \"$BRIDGE_ADDRESS\"|" "$CONFIG_FILE"
  sed -i '' "s|polygonZkEVMGlobalExitRootAddress = \"[^\"]*\"|polygonZkEVMGlobalExitRootAddress = \"$GLOBAL_EXIT_ROOT_ADDRESS\"|" "$CONFIG_FILE"
  sed -i '' "s|polygonZkEVMAddress = \"[^\"]*\"|polygonZkEVMAddress = \"$ROLLUP_ADDRESS\"|" "$CONFIG_FILE"
  
  echo "Successfully updated contract address parameters in cdk-node-config.toml"
else
  echo "Error: Config file $CONFIG_FILE does not exist"
  exit 1
fi

# Replace rollup-manager-contract,polygon-zkevm-global-exit-root-v2-contract
echo "Updating contract address parameters in agglayer-config.toml..."
AGGLAYER_CONFIG_FILE="./test-pp/config/agglayer-config.toml"
if [ -f "$AGGLAYER_CONFIG_FILE" ]; then
  # Use sed to replace contract address values in the config file
  sed -i '' "s|rollup-manager-contract = \"[^\"]*\"|rollup-manager-contract = \"$ROLLUP_MANAGER_ADDRESS\"|" "$AGGLAYER_CONFIG_FILE"
  sed -i '' "s|polygon-zkevm-global-exit-root-v2-contract = \"[^\"]*\"|polygon-zkevm-global-exit-root-v2-contract = \"$GLOBAL_EXIT_ROOT_ADDRESS\"|" "$AGGLAYER_CONFIG_FILE"
  echo "Successfully updated contract address parameters in agglayer-config.toml:"
  echo "rollup-manager-contract = $ROLLUP_MANAGER_ADDRESS"
  echo "polygon-zkevm-global-exit-root-v2-contract = $GLOBAL_EXIT_ROOT_ADDRESS"
else
  echo "Error: Config file $AGGLAYER_CONFIG_FILE does not exist"
  exit 1
fi

# 从deploy_output.json读取deploymentRollupManagerBlockNumber
echo "read deploy_output.json deploymentRollupManagerBlockNumber..."
DEPLOYMENT_ROLLUP_MANAGER_BLOCK_NUMBER=$(grep -o '"deploymentRollupManagerBlockNumber": [0-9]*' "$DEPLOY_OUTPUT_PATH" | cut -d' ' -f2)

# 检查是否成功获取到区块号
if [ -z "$DEPLOYMENT_ROLLUP_MANAGER_BLOCK_NUMBER" ]; then
  echo "Error: Unable to extract deploymentRollupManagerBlockNumber from deploy_output.json"
  exit 1
fi

echo "deploymentRollupManagerBlockNumber: $DEPLOYMENT_ROLLUP_MANAGER_BLOCK_NUMBER"

echo "update test.genesis.config.json rollupCreationBlockNumber..."
GENESIS_CONFIG_FILE="./test-pp/config/test.genesis.config.json"
if [ -f "$GENESIS_CONFIG_FILE" ]; then
  sed -i '' "s|\"rollupCreationBlockNumber\": [0-9]*|\"rollupCreationBlockNumber\": $DEPLOYMENT_ROLLUP_MANAGER_BLOCK_NUMBER|" "$GENESIS_CONFIG_FILE"
  
  echo "update DEPLOYMENT_ROLLUP_MANAGER_BLOCK_NUMBER: $DEPLOYMENT_ROLLUP_MANAGER_BLOCK_NUMBER"
else
  echo "Error: Config file $GENESIS_CONFIG_FILE does not exist"
  exit 1
fi

# Replace rollup-manager-contract,polygon-zkevm-global-exit-root-v2-contract
echo "Updating contract address parameters in agglayer-config.toml..."
AGGLAYER_CONFIG_FILE="./test-pp/config/agglayer-config.toml"
if [ -f "$AGGLAYER_CONFIG_FILE" ]; then
  # Use sed to replace contract address values in the config file
  sed -i '' "s|rollup-manager-contract = \"[^\"]*\"|rollup-manager-contract = \"$ROLLUP_MANAGER_ADDRESS\"|" "$AGGLAYER_CONFIG_FILE"
  sed -i '' "s|polygon-zkevm-global-exit-root-v2-contract = \"[^\"]*\"|polygon-zkevm-global-exit-root-v2-contract = \"$GLOBAL_EXIT_ROOT_ADDRESS\"|" "$AGGLAYER_CONFIG_FILE"
  echo "Successfully updated contract address parameters in agglayer-config.toml:"
  echo "rollup-manager-contract = $ROLLUP_MANAGER_ADDRESS"
  echo "polygon-zkevm-global-exit-root-v2-contract = $GLOBAL_EXIT_ROOT_ADDRESS"
else
  echo "Error: Config file $AGGLAYER_CONFIG_FILE does not exist"
  exit 1
fi

echo "Initialization script completed!"

cd $BASE_DIR
make run
