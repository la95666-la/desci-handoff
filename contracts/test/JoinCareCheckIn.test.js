const { expect } = require("chai");
const { ethers, upgrades } = require("hardhat");
const { time } = require("@nomicfoundation/hardhat-network-helpers");
const { anyValue } = require("@nomicfoundation/hardhat-chai-matchers/withArgs");

// Self-paid model: users call checkIn() themselves. No EIP-712, no relayer.

describe("JoinCareCheckIn", function () {
    async function deployFixture() {
        const [owner, other, user, attacker] = await ethers.getSigners();

        const JoinCareCheckIn = await ethers.getContractFactory("JoinCareCheckIn");
        const checkIn = await upgrades.deployProxy(
            JoinCareCheckIn,
            [],
            { kind: "uups", initializer: "initialize" }
        );

        return { checkIn, owner, other, user, attacker };
    }

    async function epochDayNow() {
        const now = BigInt(await time.latest());
        return now / 86400n;
    }

    describe("Deployment", function () {
        it("owner 为部署者", async function () {
            const { checkIn, owner } = await deployFixture();
            expect(await checkIn.owner()).to.equal(owner.address);
        });

        it("初始未暂停", async function () {
            const { checkIn } = await deployFixture();
            expect(await checkIn.paused()).to.equal(false);
        });

        it("不可重复 initialize", async function () {
            const { checkIn } = await deployFixture();
            await expect(checkIn.initialize())
                .to.be.revertedWithCustomError(checkIn, "InvalidInitialization");
        });
    });

    describe("checkIn (self-paid)", function () {
        it("任意用户可以调 checkIn，发出 UserCheckedIn 事件且天数是 UTC 天数", async function () {
            const { checkIn, user } = await deployFixture();
            const day = await epochDayNow();

            await expect(checkIn.connect(user).checkIn())
                .to.emit(checkIn, "UserCheckedIn")
                .withArgs(user.address, day, anyValue);

            expect(await checkIn.checkedDay(user.address, day)).to.equal(true);
            expect(await checkIn.hasCheckedInToday(user.address)).to.equal(true);
        });

        it("同一 UTC 日再签 revert", async function () {
            const { checkIn, user } = await deployFixture();
            await checkIn.connect(user).checkIn();

            await expect(checkIn.connect(user).checkIn())
                .to.be.revertedWith("Already checked in");
        });

        it("跨天后可以再签", async function () {
            const { checkIn, user } = await deployFixture();
            await checkIn.connect(user).checkIn();

            // 前进 1 UTC 天
            await time.increase(86400);

            await expect(checkIn.connect(user).checkIn())
                .to.emit(checkIn, "UserCheckedIn");

            const dayNow = await epochDayNow();
            expect(await checkIn.checkedDay(user.address, dayNow)).to.equal(true);
            expect(await checkIn.checkedDay(user.address, dayNow - 1n)).to.equal(true);
        });

        it("不同用户同一天各自独立可签", async function () {
            const { checkIn, user, other } = await deployFixture();
            await expect(checkIn.connect(user).checkIn()).to.emit(checkIn, "UserCheckedIn");
            await expect(checkIn.connect(other).checkIn()).to.emit(checkIn, "UserCheckedIn");
        });

        it("event 只有 3 个参数（user, day, timestamp）", async function () {
            const { checkIn, user } = await deployFixture();
            const tx = await checkIn.connect(user).checkIn();
            const receipt = await tx.wait();
            const log = receipt.logs.find((l) => {
                try {
                    return checkIn.interface.parseLog(l)?.name === "UserCheckedIn";
                } catch { return false; }
            });
            const parsed = checkIn.interface.parseLog(log);
            expect(parsed.args.length).to.equal(3);
            expect(parsed.args.user).to.equal(user.address);
        });

        it("hasCheckedInToday 在跨天后回到 false", async function () {
            const { checkIn, user } = await deployFixture();
            await checkIn.connect(user).checkIn();
            expect(await checkIn.hasCheckedInToday(user.address)).to.equal(true);

            await time.increase(86400);
            expect(await checkIn.hasCheckedInToday(user.address)).to.equal(false);
        });
    });

    describe("Pause controls", function () {
        it("暂停后拒绝 checkIn", async function () {
            const { checkIn, owner, user } = await deployFixture();
            await checkIn.connect(owner).pause();

            await expect(checkIn.connect(user).checkIn())
                .to.be.revertedWithCustomError(checkIn, "EnforcedPause");
        });

        it("恢复后可以继续 checkIn", async function () {
            const { checkIn, owner, user } = await deployFixture();
            await checkIn.connect(owner).pause();
            await checkIn.connect(owner).unpause();

            await expect(checkIn.connect(user).checkIn())
                .to.emit(checkIn, "UserCheckedIn");
        });

        it("非 owner 不能 pause / unpause", async function () {
            const { checkIn, attacker } = await deployFixture();
            await expect(checkIn.connect(attacker).pause())
                .to.be.revertedWithCustomError(checkIn, "OwnableUnauthorizedAccount");
            await expect(checkIn.connect(attacker).unpause())
                .to.be.revertedWithCustomError(checkIn, "OwnableUnauthorizedAccount");
        });
    });

    describe("ownership safety", function () {
        it("owner cannot renounce and non-owner remains unauthorized", async function () {
            const { checkIn, attacker } = await deployFixture();
            await expect(checkIn.renounceOwnership())
                .to.be.revertedWithCustomError(checkIn, "OwnershipRenunciationDisabled");
            await expect(checkIn.connect(attacker).renounceOwnership())
                .to.be.revertedWithCustomError(checkIn, "OwnableUnauthorizedAccount")
                .withArgs(attacker.address);
        });
    });

    describe("rescueERC20", function () {
        async function deployMockToken() {
            const MockERC20 = await ethers.getContractFactory("MockERC20");
            return await MockERC20.deploy("TEST", "TST", 18);
        }

        it("owner 可提走误发到合约的 ERC20", async function () {
            const { checkIn, owner, other } = await deployFixture();
            const token = await deployMockToken();
            const amount = ethers.parseEther("100");
            await token.mint(await checkIn.getAddress(), amount);

            await expect(checkIn.connect(owner).rescueERC20(await token.getAddress(), other.address, amount))
                .to.emit(checkIn, "TokensRescued")
                .withArgs(await token.getAddress(), other.address, amount);
            expect(await token.balanceOf(other.address)).to.equal(amount);
        });

        it("非 owner 调用被 reject", async function () {
            const { checkIn, attacker, other } = await deployFixture();
            const token = await deployMockToken();
            await expect(
                checkIn.connect(attacker).rescueERC20(await token.getAddress(), other.address, 1)
            ).to.be.revertedWithCustomError(checkIn, "OwnableUnauthorizedAccount");
        });

        it("recipient 为 zero-address 时 revert", async function () {
            const { checkIn, owner } = await deployFixture();
            const token = await deployMockToken();
            await expect(
                checkIn.connect(owner).rescueERC20(await token.getAddress(), ethers.ZeroAddress, 1)
            ).to.be.revertedWith("Invalid recipient");
        });
    });

    describe("UUPS upgrade", function () {
        it("升级后 proxy 地址不变、checkedDay 状态保留", async function () {
            const { checkIn, user } = await deployFixture();
            const proxyAddr = await checkIn.getAddress();
            const day = await epochDayNow();

            await checkIn.connect(user).checkIn();
            expect(await checkIn.checkedDay(user.address, day)).to.equal(true);

            // 升级到自身（模拟未来 V3）；plugin 会做 storage 兼容性校验
            const JoinCareCheckIn = await ethers.getContractFactory("JoinCareCheckIn");
            const upgraded = await upgrades.upgradeProxy(proxyAddr, JoinCareCheckIn);
            await upgraded.waitForDeployment();

            expect(await upgraded.getAddress()).to.equal(proxyAddr);
            expect(await upgraded.checkedDay(user.address, day)).to.equal(true);
            // 依然拒绝当日重签
            await expect(upgraded.connect(user).checkIn())
                .to.be.revertedWith("Already checked in");
        });

        it("非 owner 不能升级", async function () {
            const { checkIn, attacker } = await deployFixture();
            // 直接调 upgradeToAndCall 应被 _authorizeUpgrade 拒绝
            const fakeImpl = ethers.Wallet.createRandom().address;
            await expect(checkIn.connect(attacker).upgradeToAndCall(fakeImpl, "0x"))
                .to.be.revertedWithCustomError(checkIn, "OwnableUnauthorizedAccount");
        });
    });
});
