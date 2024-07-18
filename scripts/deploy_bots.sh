# deploy flux default bots
# usage: ./scripts/deploy_bots.sh local
network=${1:-"local"}
yes 12345678 | go run examples/chain/21_MsgConfigStrategy/example.go $network
yes 12345678 | go run examples/chain/24_ConfigIntentSolver/example.go $network
yes 12345678 | go run examples/chain/26_MsgConfigCron/example.go $network
yes 12345678 | go run examples/chain/36_ConfigAmmSolver/example.go $network
