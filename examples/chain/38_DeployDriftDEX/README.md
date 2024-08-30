# Drift protocol v2 integration

Steps to update program keys, binaries and idls in this Drift integration example

1. Install cargo, rust and anchor CLI

```
curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh
rustup install 1.76.0
rustup default 1.76.0
cargo install cbindgen

sh -c "$(curl -sSfL https://release.solana.com/v1.18.22/install)"
```

To install anchor CLI, see https://www.anchor-lang.com/docs/installation

2. Clone the repo

```
git clone https://github.com/drift-labs/protocol-v2
```

3. Build locally

```
cd path/to/protocol-v2 && anchor build
```

4. Copy artifacts

Copy files in `protocol-v2/target/deploy` to `artifacts/`
Copy files in `protocol-v2/target/idl` to `idl/`

5. Generate go client (optional)

Install anchor-go

```
go install https://github.com/gagliardetto/anchor-go
```

Generate
```
anchor-go -src=./idl/drift.json -dst=<destination_folder>
```

There is pre-generated drift client at `sdk-go/client/svm/drift`, which contains some necessary instructions being used in examples
