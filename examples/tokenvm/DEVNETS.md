## Deploying Devnets
_In the world of Avalanche, we refer to short-lived, test Subnets as Devnets._

### Step 1: Install `avalanche-ops`
[avalanche-ops](https://github.com/ava-labs/avalanche-ops) provides a command-line
interface to setup nodes and install Custom VMs on both custom networks and on
Fuji using [AWS CloudFormation](https://aws.amazon.com/cloudformation/).

You can view a full list of pre-built binaries on the [latest release
page](https://github.com/ava-labs/avalanche-ops/releases/tag/latest).

#### Option 1: Install `avalanche-ops` on Mac
```bash
rm -f /tmp/avalancheup-aws
wget "https://github.com/ava-labs/avalanche-ops/releases/download/latest/avalancheup-aws.aarch64-apple-darwin"
mv ./avalancheup-aws.aarch64-apple-darwin /tmp/avalancheup-aws
chmod +x /tmp/avalancheup-aws
/tmp/avalancheup-aws --help
```

#### Option 2: Install `avalanche-ops` on Linux
```bash
rm -f /tmp/avalancheup-aws
wget "https://github.com/ava-labs/avalanche-ops/releases/download/latest/avalancheup-aws.x86_64-linux-gnu"
mv ./avalancheup-aws.x86_64-linux-gnu /tmp/avalancheup-aws
chmod +x /tmp/avalancheup-aws
/tmp/avalancheup-aws --help
```

### Step 2: Install and Configure `aws-cli`
Next, install the [AWS CLI](https://docs.aws.amazon.com/cli/latest/userguide/getting-started-install.html). This is used to
manipulate CloudFormation.

Once you've installed the AWS CLI, run `aws configure` to set the access key to
use while deploying your devnet.

### Step 3: Install `token-cli`
```bash
export ARCH_TYPE=$(uname -m)
echo ${ARCH_TYPE}
export OS_TYPE=$(uname | tr '[:upper:]' '[:lower:]')
echo ${OS_TYPE}
export HYPERSDK_VERSION="0.0.4"
rm -f ./tmp/token-cli
wget "https://github.com/ava-labs/hypersdk/releases/download/v${HYPERSDK_VERSION}/hypersdk_${HYPERSDK_VERSION}_${OS_TYPE}_${ARCH_TYPE}.tar.gz"
mkdir tmp-hypersdk
tar -xvf hypersdk_${HYPERSDK_VERSION}_${OS_TYPE}_${ARCH_TYPE}.tar.gz -C tmp-hypersdk
rm -rf hypersdk_${HYPERSDK_VERSION}_${OS_TYPE}_${ARCH_TYPE}.tar.gz
mv tmp-hypersdk/token-cli /tmp/token-cli
rm -rf tmp-hypersdk
```

### Step 4: Download `tokenvm`
`tokenvm` is currently compiled with `GOAMD=v1`. If you want to significantly
improve the performance of cryptographic operations, you should consider
building with [`v3+`](https://github.com/golang/go/wiki/MinimumRequirements#amd64).

```bash
export HYPERSDK_VERSION="0.0.4"
rm -f ./tmp/tokenvm
wget "https://github.com/ava-labs/hypersdk/releases/download/v${HYPERSDK_VERSION}/hypersdk_${HYPERSDK_VERSION}_linux_amd64.tar.gz"
mkdir tmp-hypersdk
tar -xvf hypersdk_${HYPERSDK_VERSION}_linux_amd64.tar.gz -C tmp-hypersdk
rm -rf hypersdk_${HYPERSDK_VERSION}_linux_amd64.tar.gz
mv tmp-hypersdk/tokenvm /tmp/tokenvm
rm -rf tmp-hypersdk
```

*Note*: We must install the linux build because that is compatible with the AWS
deployment target. If you use Graivtron CPUs, make sure to use `arm64` here.


### Step 5: Plan Local Network Deploy

Now we can spin up a new network of 6 nodes with some defaults:
- `avalanche-ops` supports [Graviton-based processors](https://aws.amazon.com/ec2/graviton/) (ARM64). Use `--arch-type arm64` to run nodes in ARM64 CPUs.
- `avalanche-ops` supports [EC2 Spot instances](https://aws.amazon.com/ec2/spot/) for cost savings. Use `--instance-mode=spot` to run instances in spot mode.

```bash
# launch/download all artifacts for x86 platform
/tmp/avalancheup-aws default-spec \
--arch-type amd64 \
--rust-os-type ubuntu20.04 \
--anchor-nodes 3 \
--non-anchor-nodes 3 \
--region us-west-2 \
--instance-mode=on-demand \
--instance-size=2xlarge \
--ip-mode=elastic \
--metrics-fetch-interval-seconds 60 \
--network-name custom \
--keys-to-generate 5
```

The `default-spec` (and `apply`) command only provisions nodes, not Custom VMs.
The Subnet and Custom VM installation are done via `install-subnet-chain` sub-commands that follow.

#### Using Custom `avalanchego` Binaries
The default command above will download the `avalanchego` public release binaries from GitHub.
To test your own binaries, use the following flags to upload to S3. These binaries must be built
for the target remote machine platform (e.g., build for `aarch64` and Linux to run them in
Graviton processors):

```bash
avalancheup-aws default-spec \
...
--upload-artifacts-aws-volume-provisioner-local-bin ${AWS_VOLUME_PROVISIONER_BIN_PATH} \
--upload-artifacts-aws-ip-provisioner-local-bin ${AWS_IP_PROVISIONER_BIN_PATH} \
--upload-artifacts-avalanche-telemetry-cloudwatch-local-bin ${AVALANCHE_TELEMETRY_CLOUDWATCH_BIN_PATH} \
--upload-artifacts-avalanched-aws-local-bin ${AVALANCHED_AWS_BIN_PATH} \
--upload-artifacts-avalanchego-local-bin ${AVALANCHEGO_BIN_PATH} \
```

It is recommended to specify your own artifacts to avoid flaky github release page downloads.

#### Restricting Instance Types
By default, `avalanche-ops` will attempt to use all available instance types in
a region in your cluster. If you'd like to restrict which instance types are used,
edit the output `aops-custom-****-***.yaml` file before deploying:
```yaml
machine:
  anchor_nodes: 3
  non_anchor_nodes: 3
  arch_type: amd64
  rust_os_type: ubuntu20.04
  instance_types:
  - t3a.2xlarge
  - t3.2xlarge
  - t2.2xlarge
  - c6a.2xlarge
  - m6a.2xlarge
  - m5.2xlarge
  - c5.2xlarge
```

### Step 6: Apply Local Network Deploy

The `default-spec` command itself does not create any resources, and instead outputs the following `apply` and `delete` commands that you can copy and paste:

```bash
# first make sure you have access to the AWS credential
aws sts get-caller-identity

# "avalancheup-aws apply" would not work...
# An error occurred (ExpiredToken) when calling the GetCallerIdentity operation: The security token included in the request is expired

# "avalancheup-aws apply" will work with the following credential
# {
#   "UserId": "...:abc@abc.com",
#   "Account": "...",
#   "Arn": "arn:aws:sts::....:assumed-role/name/abc@abc.com"
# }
```

```bash
# run the following to create resources
vi /home/ubuntu/aops-custom-****-***.yaml

avalancheup-aws apply \
--spec-file-path /home/ubuntu/aops-custom-****-***.yaml

# run the following to delete resources
avalancheup-aws delete \
--delete-cloudwatch-log-group \
--delete-s3-objects \
--delete-ebs-volumes \
--delete-elastic-ips \
--spec-file-path /home/ubuntu/aops-custom-****-***.yaml
```

That is, `apply` creates AWS resources, whereas `delete` destroys after testing is done.

If you see the following command, that is expected (it means `avalanche-ops` is
reusing a S3 bucket it previously created):
```
[2023-03-23T23:49:01Z WARN  aws_manager::s3] bucket already exists so returning early (original error 'service error')
```

*SECURITY*: By default, the SSH and HTTP ports are open to public. Once you complete provisioning the nodes, go to EC2 security
group to restrict the inbound rules to your IP.

*NOTE*: In rare cases, you may encounter [aws-sdk-rust#611](https://github.com/awslabs/aws-sdk-rust/issues/611)
where AWS SDK call hangs, which blocks node bootstraps. If a node takes too long to start, connect to that
instance (e..g, use SSM sesson from your AWS console), and restart the agent with the command `sudo systemctl restart avalanched-aws`.

### Step 7: Generate Configs

Now that the network and nodes are up, let's install two subnets, each of which runs its own `tokenvm`. Use the following commands to
do so (similar to [scripts/run.sh](./scripts/run.sh)):

```bash
rm -rf /tmp/avalanche-ops

/tmp/avalancheup-aws subnet-config \
--log-level=info \
--proposer-min-block-delay 0 \
--file-path /tmp/avalanche-ops/subnet-config.json

cat <<EOF > /tmp/avalanche-ops/allocations.json
[{"address":"token1rvzhmceq997zntgvravfagsks6w0ryud3rylh4cdvayry0dl97nsjzf3yp", "balance":1000000000000}]
EOF
rm -f /tmp/avalanche-ops/tokenvm-genesis.json
/tmp/token-cli genesis generate /tmp/avalanche-ops/allocations.json \
--genesis-file /tmp/avalanche-ops/tokenvm-genesis.json
cat /tmp/avalanche-ops/tokenvm-genesis.json

cat <<EOF > /tmp/avalanche-ops/tokenvm-chain-config.json
{
  "mempoolSize": 10000000,
  "mempoolPayerSize": 10000000,
  "mempoolExemptPayers":["token1rvzhmceq997zntgvravfagsks6w0ryud3rylh4cdvayry0dl97nsjzf3yp"],
  "streamingBacklogSize": 10000000,
  "trackedPairs":["*"],
  "logLevel": "info",
  "decisionsPort": 9652,
  "blocksPort": 9653,
}
EOF
cat /tmp/avalanche-ops/tokenvm-chain-config.json
```

### Step 8: Install Chains
You can run the following commands to spin up 2 `tokenvm` Devnets. Make sure to
replace the `***` fields, IP addresses, key, and `node-ids-to-instance-ids` with your own data:
```bash
/tmp/avalancheup-aws install-subnet-chain \
--log-level info \
--region us-west-2 \
--s3-bucket <TODO> \
--s3-key-prefix <TODO> \
--ssm-doc <TODO> \
--chain-rpc-url <TODO> \
--key <TODO> \
--primary-network-validate-period-in-days 16 \
--subnet-validate-period-in-days 14 \
--subnet-config-local-path /tmp/avalanche-ops/subnet-config.json \
--subnet-config-remote-dir /data/avalanche-configs/subnets \
--vm-binary-local-path /tmp/tokenvm \
--vm-binary-remote-dir /data/avalanche-plugins \
--chain-name tokenvm1 \
--chain-genesis-path /tmp/avalanche-ops/tokenvm-genesis.json \
--chain-config-local-path /tmp/avalanche-ops/tokenvm-chain-config.json \
--chain-config-remote-dir /data/avalanche-configs/chains \
--avalanchego-config-remote-path /data/avalanche-configs/config.json \
--node-ids-to-instance-ids <TODO>
```

### Step 9: Initialize `token-cli`
You can import the demo key and the network configuration from `avalanche-ops`
using the following commands:
```bash
/tmp/token-cli key import demo.pk
/tmp/token-cli chain import-ops <chainID> <avalanche-ops spec file path>
/tmp/token-cli key balance --check-all-chains
```

### Step 10: Start Integrated Block Explorer
To view activity on each Subnet you created, you can run the `token-cli`'s
integrated block explorer. To do this, run the following command:
```bash
/tmp/token-cli chain watch --hide-txs
```

If you don't plan to load test the devnet, you may wush to just run the
following command to get additional transaction details:
```bash
/tmp/token-cli chain watch
```

### Step 11: Run Load
Once the network information is imported, you can then run the following
command to drive an arbitrary amount of load:
```bash
/tmp/token-cli spam run
```

### [OPTIONAL] Step 12: Deploy Another Subnet
To test Avalanche Warp Messaging, you must be running at least 2 Subnets. To do
so, just replicate the command you ran above with a different `--chain-name` (and
a different set of validators):
```bash
/tmp/avalancheup-aws install-subnet-chain \
--log-level info \
--region us-west-2 \
--s3-bucket <TODO> \
--s3-key-prefix <TODO> \
--ssm-doc <TODO> \
--chain-rpc-url <TODO> \
--key <TODO> \
--primary-network-validate-period-in-days 16 \
--subnet-validate-period-in-days 14 \
--subnet-config-local-path /tmp/avalanche-ops/subnet-config.json \
--subnet-config-remote-dir /data/avalanche-configs/subnets \
--vm-binary-local-path /tmp/tokenvm \
--vm-binary-remote-dir /data/avalanche-plugins \
--chain-name tokenvm2 \
--chain-genesis-path /tmp/avalanche-ops/tokenvm-genesis.json \
--chain-config-local-path /tmp/avalanche-ops/tokenvm-chain-config.json \
--chain-config-remote-dir /data/avalanche-configs/chains \
--avalanchego-config-remote-path /data/avalanche-configs/config.json \
--node-ids-to-instance-ids <TODO>
```

## Deploy TokenVM in Fuji network with avalanche-ops
Same as above, except you use `--network-name fuji` and do not need anchor nodes:

```bash
avalancheup-aws default-spec \
--arch-type amd64 \
--rust-os-type ubuntu20.04 \
--non-anchor-nodes 6 \
--region us-west-2 \
--instance-mode=on-demand \
--instance-size=2xlarge \
--ip-mode=elastic \
--metrics-fetch-interval-seconds 60 \
--network-name fuji
```

Make sure the nodes are in sync with the chain state before installing
subnets/chains with `avalancheup-aws install-subnet-chain`. You can check the status
of the nodes either via HTTP `/health` endpoints or CloudWatch logs.