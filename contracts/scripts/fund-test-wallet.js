const { ethers } = require('ethers');

const RPC = 'http://172.18.0.4:8545';
const OWNER_PK = '0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80';
const TARGET = '0x6c099660a0ee289de73466fe24f88fbfa81d3a93';
const USDT = '0x9fE46736679d2D9a65F0992F2272dE9f3c7fa6e0';

async function main() {
  const provider = new ethers.JsonRpcProvider(RPC);
  const wallet = new ethers.Wallet(OWNER_PK, provider);
  const usdt = new ethers.Contract(USDT, ['function transfer(address,uint256) returns (bool)', 'function balanceOf(address) view returns (uint256)', 'function decimals() view returns (uint8)'], wallet);
  const dec = await usdt.decimals();

  console.log('target ETH balance:', (await provider.getBalance(TARGET)).toString(), '(' + ethers.formatEther(await provider.getBalance(TARGET)) + ' ETH)');
  console.log('target USDT balance BEFORE:', (await usdt.balanceOf(TARGET)).toString());

  const usdtAmount = ethers.parseUnits('100000', dec);
  const nonce = await wallet.getNonce();
  console.log('using nonce:', nonce);
  const tx = await usdt.transfer(TARGET, usdtAmount, { nonce });
  await tx.wait();
  console.log('USDT tx:', tx.hash);

  console.log('target USDT balance AFTER:', (await usdt.balanceOf(TARGET)).toString());
}

main().then(() => process.exit(0)).catch((e) => { console.error(e.shortMessage || e.message); process.exit(1); });