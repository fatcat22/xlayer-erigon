#!/bin/bash
# 设置错误时退出
set -e

# 定义可配置参数
DOCKER_IMAGE_NAME=${DOCKER_IMAGE_NAME:-"zjg555543/geth"}
DOCKER_IMAGE_TAG=${DOCKER_IMAGE_TAG:-"pp-v5"}

echo "开始执行初始化脚本..."

# 清理docker
echo "清理所有docker容器..."
docker stop $(docker ps -aq) || true
docker rm $(docker ps -aq) || true
docker ps -a

# 启动mock l1
echo "启动zkevm-mock-l1-network..."
docker-compose up -d zkevm-mock-l1-network

# 等待几秒钟确保网络启动
echo "等待网络启动..."
sleep 5

## 准备合约deployer
echo "准备合约deployer..."
DEPLOYER_ADDRESS="0x8f8E2d6cF621f30e9a11309D6A56A876281Fd534"
DEPLOYER_PRIVATE_KEY="0x815405dddb0e2a99b12af775fd2929e526704e1d1aea6a0b4e74dc33e2f7fcd2"
DEPLOYER_MNEMONIC="moment wine false celery win galaxy glide thumb tail setup choose city"

RICH_ADDRESS="0x14dC79964da2C08b23698B3D3cc7Ca32193d9955"
RICH_PRIVATE_KEY="0x4bbbf85ce3377467afe5d46f804f221813b2bb87f24d81f60f1fcdbf7cbf4356"

### 查看余额
echo "查看富地址余额..."
cast rpc eth_getBalance $RICH_ADDRESS latest

### 给deployer打钱
echo "给deployer打钱..."
cast send -f $RICH_ADDRESS --private-key $RICH_PRIVATE_KEY --value 3ether --legacy $DEPLOYER_ADDRESS

# 重置合约环境
echo "克隆合约代码库..."
if [ ! -d "./agglayer-contracts" ]; then
  git clone -b v10.0.0-rc.6 https://github.com/agglayer/agglayer-contracts.git
fi

cd ./agglayer-contracts

# 创建.env文件
echo "创建.env文件..."
cat > .env << EOF
MNEMONIC="$DEPLOYER_MNEMONIC"
INFURA_PROJECT_ID="6d3d0adfe7c74dcb87642b37f1477aec"
ETHERSCAN_API_KEY="F948SAS2HRYQ8V8Y315RWMT1E21M3K3PTK"
EOF

cd deployment/v2

# 创建create_rollup_parameters.json
echo "创建create_rollup_parameters.json..."
cat > create_rollup_parameters.json << EOF
{
    "adminZkEVM": "$DEPLOYER_ADDRESS",
    "chainID": 195,
    "consensusContract": "PolygonPessimisticConsensus",
    "dataAvailabilityProtocol": "PolygonDataCommittee",
    "deployerPvtKey": "",
    "description": "",
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

# 创建deploy_parameters.json
echo "创建deploy_parameters.json..."
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

# 编译合约
echo "编译合约..."
cd ../../
npm i
npm run deploy:v2:localhost

# 查看genesis文件
# echo "查看genesis文件..."
# cat deployment/v2/genesis.json
# cat deployment/v2/deploy_output.json 
# cat deployment/v2/create_rollup_output.json

# 查看当前的委员会个数
# echo "查看当前的委员会个数..."
# DATA_COMMITTEE_ADDRESS="0x3bFa19E4588962D1834B2e4007F150f4447Aa9fe"
# cast call $DATA_COMMITTEE_ADDRESS 'function getAmountOfMembers() public returns (uint256)'

# 转移给Seq ERC20 token
echo "转移ERC20 token给Sequencer..."
SEQ_ADDRESS="0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266"
SEQ_PRIVATE_KEY="0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80"
TOKEN_ADDRESS="0x5FbDB2315678afecb367f032d93F642f64180aa3"
cast send --legacy --from $SEQ_ADDRESS --private-key $SEQ_PRIVATE_KEY $TOKEN_ADDRESS "transfer(address,uint256)" $SEQ_ADDRESS 1000

# 设置1个委员会
# echo "设置委员会..."
# cast send --legacy --from $DEPLOYER_ADDRESS --private-key $DEPLOYER_PRIVATE_KEY $DATA_COMMITTEE_ADDRESS 'function setupCommittee(uint256 _requiredAmountOfSignatures, string[] urls, bytes addrsBytes) returns()' 1 [http://xlayer-da:8444] $SEQ_ADDRESS

# 设置Trust RPC
echo "设置Trusted Sequencer URL..."
POE_ADDRESS="0xeb173087729c88a47568AF87b17C653039377BA6"
cast send --legacy --from $DEPLOYER_ADDRESS --private-key $DEPLOYER_PRIVATE_KEY $POE_ADDRESS "setTrustedSequencerURL(string)" "http://xlayer-rpc:8545"

# 跨链激活
echo "激活跨链功能..."
BRIDGE_ADDRESS="0x3a277Fa4E78cc1266F32E26c467F99A8eAEfF7c3"
cast send --legacy --from $DEPLOYER_ADDRESS --private-key $DEPLOYER_PRIVATE_KEY $BRIDGE_ADDRESS 'function bridgeAsset(uint32 destinationNetwork, address destinationAddress, uint256 amount, address token, bool forceUpdateGlobalExitRoot, bytes permitData) returns()' 7 0x0000000000000000000000000000000000000000 0 0x0000000000000000000000000000000000000000 true 0x

echo "查看运行中的容器..."
docker ps

# 获取容器ID
CONTAINER_ID=$(docker ps | grep zkevm-mock-l1-network | awk '{print $1}')
if [ -n "$CONTAINER_ID" ]; then
  echo "进入容器 $CONTAINER_ID..."
  docker exec -it $CONTAINER_ID /bin/sh -c "ps -ef | grep geth; kill -15 \$(ps -ef | grep geth | grep -v grep | awk '{print \$1}')"
fi

echo "------------------------------------------------------------"
echo "镜像到此位置结束，可以参考提交镜像"
echo "docker ps -a"
echo "docker commit $CONTAINER_ID ${DOCKER_IMAGE_NAME}:${DOCKER_IMAGE_TAG}"
echo "docker push ${DOCKER_IMAGE_NAME}:${DOCKER_IMAGE_TAG}"

echo "生成配置文件..."
cd ../../
go run cmd/hack/allocs/main.go ./test-pp/config/genesis.json

echo "初始化脚本执行完成！"
