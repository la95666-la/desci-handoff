'use strict';
const {ethers,network,upgrades}=require('hardhat');
const {deployFacetSet,manifest,validateLocalFixture,aggregateInterface}=require('../../fairLaunchArchitecture');
const localUpgradeFixture=require('../../validation/fair-launch-local-upgrade-manifest.json');
const STORAGE_ROOT=BigInt('0x7ebb4c309dc14c823b32233ea7cfb602ee106e1bc7e105581a751cd0bcb46700');
const HAS_PARTICIPATED_OFFSET=16n;

async function prepareUpgradeValidation(){
 if(network.name!=='hardhat')throw new Error('validation is local-only');
 const one=await deployFacetSet(),two=await deployFacetSet();
 const F=await ethers.getContractFactory('JoinCareFairLaunchV1');
 await upgrades.validateImplementation(F,{kind:'uups',constructorArgs:[await two.core.getAddress(),await two.release.getAddress(),await two.funds.getAddress(),await two.admin.getAddress(),await two.views.getAddress()],unsafeAllow:['constructor','state-variable-immutable']});
 const [owner,operator,guardian,unpause,treasury,pendingOwner,projectOwner,user1,user2]=await ethers.getSigners(),iface=await aggregateInterface();
 const Safe=await ethers.getContractFactory('FairLaunchSafeMock'),proceedsA=await Safe.deploy(),proceedsB=await Safe.deploy(),fees=await Safe.deploy();
 const Proxy=await ethers.getContractFactory('FairLaunchERC1967Proxy'),proxy=await Proxy.deploy(await one.dispatcher.getAddress(),iface.encodeFunctionData('initialize',[owner.address,operator.address,guardian.address,unpause.address,treasury.address,await fees.getAddress(),250]));
 const c=new ethers.Contract(await proxy.getAddress(),iface,owner),Token=await ethers.getContractFactory('FairLaunchTokenMock');
 const projectA=await Token.deploy('Project A','PA',18),projectB=await Token.deploy('Project B','PB',8),paymentA=await Token.deploy('Payment A','PAYA',6),paymentB=await Token.deploy('Payment B','PAYB',18);
 const codeHash=async token=>ethers.keccak256(await ethers.provider.getCode(await token.getAddress()));
 const projectPolicyA=ethers.id('project-policy-a'),projectPolicyB=ethers.id('project-policy-b'),paymentPolicyA=ethers.id('payment-policy-a'),paymentPolicyB=ethers.id('payment-policy-b'),releaseDigest=ethers.id('release-policy');
 const proceedsADigest=ethers.id('proceeds-a'),proceedsBDigest=ethers.id('proceeds-b'),feeDigest=ethers.id('platform-fee-safe');
 await c.configureProjectToken(await projectA.getAddress(),[true,false,18,1,await codeHash(projectA),projectPolicyA]);
 await c.configureProjectToken(await projectB.getAddress(),[true,false,8,1,await codeHash(projectB),projectPolicyB]);
 await c.registerPaymentTokenPolicyRevision(await paymentA.getAddress(),[true,6,1,await codeHash(paymentA),paymentPolicyA]);
 await c.registerPaymentTokenPolicyRevision(await paymentB.getAddress(),[true,18,1,await codeHash(paymentB),paymentPolicyB]);
 await c.setPaymentTokenAdmission(await paymentA.getAddress(),1,true);await c.setPaymentTokenAdmission(await paymentB.getAddress(),1,true);
 await c.configureBeneficiarySafe(await proceedsA.getAddress(),true,proceedsADigest,1);await c.configureBeneficiarySafe(await proceedsB.getAddress(),true,proceedsBDigest,2);await c.configureBeneficiarySafe(await fees.getAddress(),true,feeDigest,3);await c.configureReleasePolicy(releaseDigest,1,true);
 const now=(await ethers.provider.getBlock('latest')).timestamp,start=now+30,end=now+180,saleA=ethers.parseEther('100'),saleB=100_00000000n;
 const create=async(id,projectToken,paymentToken,proceeds,proceedsDigest,proceedsVersion,sale,soft,min,max,projectDigest,paymentDigest)=>c.connect(operator).createProject([id,await projectToken.getAddress(),await paymentToken.getAddress(),projectOwner.address,await proceeds.getAddress(),proceedsDigest,proceedsVersion,await fees.getAddress(),feeDigest,3,250,sale,soft,min,max,start,end,projectDigest,1,paymentDigest,1,releaseDigest,1],[end],[10000]);
 for(const id of [1,2,3,4,6,7])await create(id,projectA,paymentA,proceedsA,proceedsADigest,1,saleA,id===2?4_000000n:1_000000n,100_000n,0n,projectPolicyA,paymentPolicyA);
 await create(5,projectB,paymentB,proceedsB,proceedsBDigest,2,saleB,ethers.parseEther('1'),ethers.parseEther('0.1'),0n,projectPolicyB,paymentPolicyB);
 await projectA.mint(projectOwner.address,saleA*6n);await projectB.mint(projectOwner.address,saleB);await projectA.connect(projectOwner).approve(await c.getAddress(),ethers.MaxUint256);await projectB.connect(projectOwner).approve(await c.getAddress(),ethers.MaxUint256);
 for(const id of [1,2,3,4,6,7])await c.connect(projectOwner).depositProjectTokens(id,saleA);for(const id of [2,3,4,6,7])await c.connect(operator).activateProject(id);await c.connect(projectOwner).depositProjectTokens(5,saleB);await c.connect(operator).activateProject(5);
 await paymentA.mint(user1.address,10_000000n);await paymentA.mint(user2.address,10_000000n);await paymentB.mint(user2.address,ethers.parseEther('5'));await paymentA.connect(user1).approve(await c.getAddress(),ethers.MaxUint256);await paymentA.connect(user2).approve(await c.getAddress(),ethers.MaxUint256);await paymentB.connect(user2).approve(await c.getAddress(),ethers.MaxUint256);
 await network.provider.send('evm_setNextBlockTimestamp',[start+1]);
 await c.connect(user1).contribute(2,1_000000n);await c.connect(user2).contribute(2,1_500000n);
 await c.connect(user1).contribute(3,1_000000n);await c.connect(user2).contribute(3,500_000n);
 await c.connect(user1).contribute(4,1_000000n);await c.connect(user2).contribute(4,500_000n);
 await c.connect(user2).contribute(5,ethers.parseEther('2'));
 await c.connect(user1).contribute(6,200_000n);await c.connect(user1).contribute(7,300_000n);await c.connect(user2).contribute(7,400_000n);await c.connect(operator).cancelProject(7,ethers.id('cancel-seven'),ethers.id('cancel-seven-evidence'));
 await network.provider.send('evm_setNextBlockTimestamp',[end]);for(const id of [2,3,4,5])await c.endProject(id);
 for(let i=0;i<20;i++)await network.provider.send('evm_mine');for(const id of [4,5]){const p=await c.getProject(id);await c.connect(operator).releaseSuccessfulProject(id,p.settlementBlockNumber,p.settlementAnchorBlockNumber,p.settlementAnchorBlockHash,ethers.id(`ended-${id}`),0,ethers.id(`manifest-${id}`));}
 await c.transferOwnership(pendingOwner.address);
 await c.connect(guardian).pauseContributions(ethers.id('pause-c'),ethers.id('pause-c-evidence'));await c.connect(guardian).pausePayouts(ethers.id('pause-p'),ethers.id('pause-p-evidence'));await c.connect(guardian).pauseClaims(BigInt(end+1000),ethers.id('pause-cl'),ethers.id('pause-cl-evidence'));
 const tracked={projects:[1,2,3,4,5,6,7],users:[user1,user2],tokens:[projectA,projectB,paymentA,paymentB],safes:[proceedsA,proceedsB,fees]};
 await assertConservation(c,tracked);
 const before=await snapshot(c,tracked);
 const actualUpgradeManifest=await manifest(two);validateLocalFixture(actualUpgradeManifest,localUpgradeFixture);
 return {c,two,tracked,actualUpgradeManifest,before,owner,pendingOwner,unpause,projectOwner,user1,user2,proceedsA,proceedsB,fees};
}
async function completeUpgradeValidation(context,approval){
 const {c,two,tracked,actualUpgradeManifest,before,owner,pendingOwner,unpause,projectOwner,user1,user2,proceedsA,proceedsB,fees}=context;
 await c.upgradeToAndCall(await two.dispatcher.getAddress(),'0x');
 const after=await snapshot(c,tracked);if(before!==after)throw new Error('observable upgrade state/invariant snapshot changed');
 const parsed=JSON.parse(after);if(JSON.stringify(parsed.lifecycleStates)!==JSON.stringify(['0','4','2','3','3','1','5']))throw new Error(`upgrade fixture lifecycle coverage lost: ${parsed.lifecycleStates}`);
 if(parsed.owner.toLowerCase()!==owner.address.toLowerCase()||parsed.pendingOwner.toLowerCase()!==pendingOwner.address.toLowerCase())throw new Error('owner/pending owner not preserved');
 if(!parsed.pauseDomains.contributionsPaused||!parsed.pauseDomains.payoutsPaused||!parsed.pauseDomains.claimsPaused)throw new Error('all pause domains were not preserved');
 await impersonate(unpause.address);const unpauseSigner=await ethers.getSigner(unpause.address);await c.connect(unpauseSigner).unpauseContributions(ethers.id('unpause-c'));await c.connect(unpauseSigner).unpausePayouts(ethers.id('unpause-p'));await c.connect(unpauseSigner).unpauseClaims(ethers.id('unpause-cl'));
 await c.refundFor(2,user1.address);await c.refundFor(2,user2.address);await c.connect(projectOwner).withdrawFailedProjectTokens(2);await c.refundFor(7,user1.address);await c.refundFor(7,user2.address);await c.connect(projectOwner).withdrawFailedProjectTokens(7);
 await c.claimFor(4,user1.address);await c.claimFor(4,user2.address);await c.claimFor(5,user2.address);
 for(const safe of [proceedsA,proceedsB,fees])await impersonate(await safe.getAddress());
 await c.connect(await ethers.getSigner(await proceedsA.getAddress())).withdrawProjectProceeds(4);await c.connect(await ethers.getSigner(await proceedsB.getAddress())).withdrawProjectProceeds(5);await c.connect(await ethers.getSigner(await fees.getAddress())).withdrawPlatformFee(4);await c.connect(await ethers.getSigner(await fees.getAddress())).withdrawPlatformFee(5);
 await assertConservation(c,tracked);
 const p2=await c.getProject(2),p4=await c.getProject(4),p5=await c.getProject(5),p7=await c.getProject(7),u21=await c.getPosition(2,user1.address),u22=await c.getPosition(2,user2.address),u71=await c.getPosition(7,user1.address),u72=await c.getPosition(7,user2.address);
 if(!u21.refunded||!u22.refunded||!p2.failedProjectTokensWithdrawn||!u71.refunded||!u72.refunded||!p7.failedProjectTokensWithdrawn)throw new Error('post-upgrade failed/cancelled refund or token withdrawal incomplete');
 if(!p4.projectProceedsWithdrawn||!p4.platformFeeWithdrawn||!p5.projectProceedsWithdrawn||!p5.platformFeeWithdrawn)throw new Error('post-upgrade proceeds/fee withdrawal incomplete');
 if((await c.getPosition(4,user1.address)).claimedTokens===0n||(await c.getPosition(4,user2.address)).claimedTokens===0n||(await c.getPosition(5,user2.address)).claimedTokens===0n)throw new Error('post-upgrade claims incomplete');
 console.log(JSON.stringify({upgradeSafe:true,observableStateSnapshotPreserved:true,lifecycleStates:{pending:0,active:1,successfulPendingRelease:2,successful:3,failed:4,cancelled:5},exactVestingSchedulesPreserved:true,multipleUsers:true,multipleTokens:true,sharedTokenProjects:true,ownerAndPendingOwnerPreserved:true,platformFeeConfigAndSafesPreserved:true,hasParticipatedPreserved:true,allPauseDomainsPreserved:true,postUpgradeActions:['failed-refunds','cancelled-refunds','failed-project-token-withdrawal','cancelled-project-token-withdrawal','claims','project-proceeds-withdrawals','platform-fee-withdrawals'],conservationAssertionsAcrossAllSixStates:true,approval},null,2));
 return {approval,actualUpgradeManifest};
}
async function snapshot(c,tracked){
 const projects=[],vesting=[],positions=[],participation=[];for(const id of tracked.projects){const p=await c.getProject(id);projects.push([...p].map(String));const schedule=await c.getVestingSchedule(id);vesting.push({id,times:[...schedule[0]].map(String),bps:[...schedule[1]].map(String)});for(const user of tracked.users){positions.push({id,user:user.address,value:[...await c.getPosition(id,user.address)].map(String)});participation.push({id,user:user.address,value:await hasParticipated(c,id,user.address)});}}
 const tokenState=[];for(const token of tracked.tokens)tokenState.push({address:await token.getAddress(),reserve:String(await c.reservedBalance(await token.getAddress())),balance:String(await token.balanceOf(await c.getAddress()))});
 const safeState=[];for(const safe of tracked.safes)safeState.push({address:await safe.getAddress(),config:[...await c.beneficiarySafeConfig(await safe.getAddress())].map(String)});
 const [feeBps,feeReceiver]=await c.platformFeeConfig(),controlWord=await ethers.provider.getStorage(await c.getAddress(),STORAGE_ROOT+4n),word=BigInt(controlWord),claimsPauseUntil=(word>>192n)&((1n<<64n)-1n);
 const policyState=[];for(const token of tracked.tokens){const a=await c.paymentTokenAdmission(await token.getAddress()),r=await c.paymentTokenPolicyRevision(await token.getAddress(),1),p=await c.projectTokenPolicy(await token.getAddress());policyState.push({address:await token.getAddress(),admission:[...a].map(String),paymentRevision:[...r].map(String),projectPolicy:[...p].map(String)});}
 return JSON.stringify({owner:await c.owner(),pendingOwner:await c.pendingOwner(),roles:{operator:await c.operator(),pauseGuardian:await c.pauseGuardian(),unpauseTimelock:await c.unpauseTimelock(),treasury:await c.treasury()},platformFee:{feeBps:String(feeBps),feeReceiver},safeState,projects,vesting,positions,participation,tokenState,policyState,releasePolicy:[...await c.releasePolicyConfig(ethers.id('release-policy'))].map(String),controlWord,pauseDomains:{contributionsPaused:((word>>176n)&255n)!==0n,payoutsPaused:((word>>184n)&255n)!==0n,claimsPauseUntil:String(claimsPauseUntil),claimsPaused:await c.isClaimsPaused()},lifecycleStates:projects.map(p=>p[13])});
}
async function hasParticipated(c,id,user){const outer=ethers.keccak256(ethers.AbiCoder.defaultAbiCoder().encode(['uint256','uint256'],[id,STORAGE_ROOT+HAS_PARTICIPATED_OFFSET]));const slot=ethers.keccak256(ethers.AbiCoder.defaultAbiCoder().encode(['address','bytes32'],[user,outer]));return BigInt(await ethers.provider.getStorage(await c.getAddress(),slot))!==0n;}
async function assertConservation(c,tracked){
 const projectLiabilities=new Map(),paymentLiabilities=new Map();const add=(map,key,value)=>map.set(key,(map.get(key)||0n)+value);
 for(const id of tracked.projects){const p=await c.getProject(id),projectToken=p.projectToken.toLowerCase(),paymentToken=p.paymentToken.toLowerCase();
  if(p.status===0n||p.status===1n||p.status===2n)add(projectLiabilities,projectToken,p.totalTokensDeposited);else if((p.status===4n||p.status===5n)&&!p.failedProjectTokensWithdrawn)add(projectLiabilities,projectToken,p.totalTokensDeposited);else if(p.status===3n){if(p.totalTokensClaimed>p.saleTokenAmount)throw new Error(`project ${id} claimed above sale amount`);add(projectLiabilities,projectToken,p.saleTokenAmount-p.totalTokensClaimed);}
  if(p.status===0n||p.status===1n||p.status===2n)add(paymentLiabilities,paymentToken,p.totalRaised);else if(p.status===4n||p.status===5n){for(const user of tracked.users){const pos=await c.getPosition(id,user.address);if(!pos.refunded)add(paymentLiabilities,paymentToken,pos.contributionAmount);}}else if(p.status===3n){const fee=p.finalTotalRaised*BigInt(p.platformFeeBps)/10000n;if(!p.projectProceedsWithdrawn)add(paymentLiabilities,paymentToken,p.finalTotalRaised-fee);if(!p.platformFeeWithdrawn)add(paymentLiabilities,paymentToken,fee);}
 }
 for(const token of tracked.tokens){const address=(await token.getAddress()).toLowerCase(),reserve=await c.reservedBalance(address),balance=await token.balanceOf(await c.getAddress()),liability=(projectLiabilities.get(address)||0n)+(paymentLiabilities.get(address)||0n);if(balance<reserve)throw new Error(`balance below reserve for ${address}`);if(reserve!==liability)throw new Error(`recorded reserve ${reserve} != computed liabilities ${liability} for ${address}`);}
}
async function impersonate(address){await network.provider.request({method:'hardhat_impersonateAccount',params:[address]});await network.provider.send('hardhat_setBalance',[address,'0x56BC75E2D63100000']);}
module.exports={prepareUpgradeValidation,completeUpgradeValidation};
