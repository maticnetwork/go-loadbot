size=$(du -s ../geth-git/test-chain-dir/bor/chaindata | awk '{print $1}')
echo $size