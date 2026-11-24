const { expect } = require("chai");
const { ethers } = require("hardhat");

/**
 * JC 平台代币测试（A3 改名 + 安全审计 0.2：mint 硬顶 / Ownable2Step）
 */
describe("JC Token", function () {
    let jc, owner, alice, safe;

    const INITIAL_SUPPLY = 1_000_000; // 100万
    const CAP = ethers.parseEther("1000000000"); // 10亿

    beforeEach(async function () {
        [owner, alice, safe] = await ethers.getSigners();
        const JC = await ethers.getContractFactory("JC");
        jc = await JC.deploy(INITIAL_SUPPLY);
    });

    describe("Deployment", function () {
        it("名称与符号", async function () {
            expect(await jc.name()).to.equal("JoinCare Coin");
            expect(await jc.symbol()).to.equal("JC");
        });

        it("初始供应铸给部署者", async function () {
            expect(await jc.totalSupply()).to.equal(ethers.parseEther("1000000"));
            expect(await jc.balanceOf(owner.address)).to.equal(ethers.parseEther("1000000"));
        });

        it("硬顶常量 = 10 亿", async function () {
            expect(await jc.MAX_SUPPLY()).to.equal(CAP);
        });

        it("初始供应超过硬顶应回退", async function () {
            const JC = await ethers.getContractFactory("JC");
            await expect(JC.deploy(1_000_000_001)).to.be.revertedWith("JC: initial supply exceeds cap");
        });
    });

    describe("Mint cap（审计 0.2 核心）", function () {
        it("owner 可在硬顶内增发", async function () {
            const amount = ethers.parseEther("1000");
            await jc.mint(alice.address, amount);
            expect(await jc.balanceOf(alice.address)).to.equal(amount);
            expect(await jc.totalSupply()).to.equal(ethers.parseEther("1000000") + amount);
        });

        it("增发超过硬顶应回退", async function () {
            const over = CAP - (await jc.totalSupply()) + 1n;
            await expect(jc.mint(alice.address, over)).to.be.revertedWith("JC: mint exceeds max supply");
        });

        it("增发到正好硬顶可以，再多 1 wei 回退", async function () {
            const remain = CAP - (await jc.totalSupply());
            await jc.mint(alice.address, remain);
            expect(await jc.totalSupply()).to.equal(CAP);
            await expect(jc.mint(alice.address, 1n)).to.be.revertedWith("JC: mint exceeds max supply");
        });

        it("非 owner 不能增发", async function () {
            await expect(jc.connect(alice).mint(alice.address, 1n))
                .to.be.revertedWithCustomError(jc, "OwnableUnauthorizedAccount");
        });
    });

    describe("Ownable2Step（防误转）", function () {
        it("transferOwnership 后原 owner 仍有效，accept 后才切换", async function () {
            await jc.transferOwnership(safe.address);
            expect(await jc.owner()).to.equal(owner.address); // 未 accept 前不变
            expect(await jc.pendingOwner()).to.equal(safe.address);

            await jc.connect(safe).acceptOwnership();
            expect(await jc.owner()).to.equal(safe.address);
        });

        it("非 pendingOwner 不能 accept", async function () {
            await jc.transferOwnership(safe.address);
            await expect(jc.connect(alice).acceptOwnership())
                .to.be.revertedWithCustomError(jc, "OwnableUnauthorizedAccount");
        });

        it("owner 迁移到新地址后，旧 owner 失去 mint 权限", async function () {
            await jc.transferOwnership(safe.address);
            await jc.connect(safe).acceptOwnership();
            await expect(jc.mint(alice.address, 1n))
                .to.be.revertedWithCustomError(jc, "OwnableUnauthorizedAccount");
            await jc.connect(safe).mint(alice.address, 1n);
            expect(await jc.balanceOf(alice.address)).to.equal(1n);
        });
    });

    describe("Burn", function () {
        it("持有人可销毁自己的代币", async function () {
            const amount = ethers.parseEther("100");
            await jc.burn(amount);
            expect(await jc.balanceOf(owner.address)).to.equal(ethers.parseEther("1000000") - amount);
        });

        it("burnFrom 需要足够授权", async function () {
            const amount = ethers.parseEther("100");
            await expect(jc.connect(alice).burnFrom(owner.address, amount))
                .to.be.revertedWith("ERC20: burn amount exceeds allowance");
            await jc.approve(alice.address, amount);
            await jc.connect(alice).burnFrom(owner.address, amount);
            expect(await jc.balanceOf(owner.address)).to.equal(ethers.parseEther("1000000") - amount);
        });
    });
});
