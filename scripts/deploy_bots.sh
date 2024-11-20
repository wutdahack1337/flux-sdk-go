# deploy flux default bots
# usage: ./scripts/deploy_bots.sh <network>
# e.g ./scripts/deploy_bots.sh local
network=${1:-"local"}
yes 12345678 | go run examples/chain/21_MsgConfigStrategy/example.go $network
yes 12345678 | go run examples/chain/24_ConfigIntentSolver/example.go $network
yes 12345678 | go run examples/chain/26_MsgConfigCron/example.go $network
yes 12345678 | go run examples/chain/36_ConfigAmmSolver/example.go $network
yes 12345678 | go run examples/chain/44_DriftSolver/1_ConfigSolver/example.go $network
yes 12345678 | go run examples/chain/45_StakingSolver/1_Config/example.go $network
