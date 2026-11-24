#!/usr/bin/env python3
"""Offline-only IDO V2 Set parameter validation and SQL rendering.

The current release has no admitted database transformation authority. Only an
the exact grammar ``PARAMS.json --dry-run`` may read the parameter file and
render review evidence. Options in any other order and ``--`` are not accepted.
Every non-dry-run invocation fails before reading parameters, RPC configuration,
Docker state, credentials, or any database input.
"""
import argparse
import json
import math
import sys
from datetime import datetime, timedelta, timezone

ALLOWED_STATUS = {"pre_launch", "Set", "warmup", "Setup"}


def to_raw(value: str, decimals: int) -> str:
    """十进制金额字符串 → 10^decimals 整数 raw（字符串精确移位，无浮点误差）"""
    s = str(value).strip()
    neg = s.startswith("-")
    if neg:
        raise ValueError(f"金额不能为负: {value}")
    if "." in s:
        ip, fp = s.split(".", 1)
    else:
        ip, fp = s, ""
    ip = ip or "0"
    if not (ip.isdigit() and (fp == "" or fp.isdigit())):
        raise ValueError(f"非法金额格式: {value}")
    if len(fp) > decimals:
        raise ValueError(f"{value} 精度超过 {decimals} 位小数，无法精确表示")
    fp = fp + "0" * (decimals - len(fp))
    raw = (ip + fp).lstrip("0") or "0"
    if raw == "0":
        raise ValueError(f"金额必须大于 0: {value}")
    return raw


def parse_time(spec: str, now: datetime) -> datetime:
    """绝对 'YYYY-MM-DDTHH:MM:SSZ'（UTC）或相对 '+90m' / '+2h' / '+30s'"""
    s = str(spec).strip()
    if s.startswith("+"):
        unit, n = s[-1], int(s[1:-1])
        delta = {"m": timedelta(minutes=n), "h": timedelta(hours=n), "s": timedelta(seconds=n), "d": timedelta(days=n)}[unit]
        return now + delta
    dt = datetime.strptime(s, "%Y-%m-%dT%H:%M:%SZ").replace(tzinfo=timezone.utc)
    return dt


def parse_offline_mode(argv):
    """Accept exactly: PARAMS.json --dry-run. Everything else is retired."""
    if len(argv) != 2 or argv[1] != "--dry-run":
        return None
    params = argv[0]
    if not params or params == "--" or params.startswith("-"):
        return None
    return params


def main():
    # Decide mode from the exact option grammar before argparse, environment, clock,
    # or parameter-file access. A token after `--` or a filename named --dry-run is
    # never treated as the offline option.
    params = parse_offline_mode(sys.argv[1:])
    if params is None:
        print("fair_launch_transformation_unavailable", file=sys.stderr)
        return 78

    ap = argparse.ArgumentParser()
    ap.add_argument("params", help="参数 JSON 文件")
    ap.add_argument("--dry-run", action="store_true", required=True, help="仅离线校验并打印 SQL，不执行")
    args = ap.parse_args([params, "--dry-run"])

    with open(args.params, encoding="utf-8") as f:
        p = json.load(f)

    now = datetime.now(timezone.utc)
    # —— 必填参数 ——
    project_id = int(p["project_id"])
    token_address = p["token_address"]
    token_decimals = int(p["token_decimals"])
    payment_symbol = p["payment_token"]            # USDT / USDC（须已在 secret.yaml v2.paymentTokens 配置）
    payment_address = p["payment_token_address"]
    payment_decimals = int(p["payment_token_decimals"])
    owner_address = p["owner_address"]
    token_price = str(p["token_price"])
    hard_cap = str(p["hard_cap"])
    soft_cap = str(p["soft_cap"])
    max_per_user = str(p["max_per_user"])
    start = parse_time(p["start_time"], now)
    end = parse_time(p["end_time"], now)
    tge_after_end = int(p.get("tge_after_end_min", 10))
    tges = p["tge"]  # [{percent: 100}] 单期或多期；第一期时间为 end+tge_after_end_min，之后按 interval_min 递推
    interval_min = int(p.get("tge_interval_min", 0))
    # 数据库 DATETIME 为无时区裸值，后端按容器本地时区解释（GORM loc=Local）。
    # 此值必须等于后端容器 TZ 与真实时间的偏移，否则创建时校验会差一个时区。
    db_tz_offset_hours = int(p.get("db_tz_offset_hours", 8))

    # —— 静态校验 ——
    errors = []
    if token_address.lower() == payment_address.lower():
        errors.append("项目币与支付币地址相同")
    if not (0 < token_decimals <= 18):
        errors.append("token_decimals 必须在 1~18")
    price_raw = hard_err = None
    try:
        price_raw = to_raw(token_price, payment_decimals)
        hard_raw = to_raw(hard_cap, payment_decimals)
        soft_raw = to_raw(soft_cap, payment_decimals)
        max_raw = to_raw(max_per_user, payment_decimals)
    except ValueError as e:
        errors.append(str(e))
    if errors:
        print("[REJECTED] 参数校验失败：", file=sys.stderr)
        for e in errors:
            print("  -", e, file=sys.stderr)
        sys.exit(2)
    if int(soft_raw) > int(hard_raw):
        errors.append(f"soft_cap {soft_cap} > hard_cap {hard_cap}")
    if int(max_raw) > int(hard_raw):
        errors.append(f"max_per_user {max_per_user} > hard_cap {hard_cap}")
    if start <= now:
        errors.append(f"start_time {start.isoformat()} 不在未来")
    if end <= start:
        errors.append("end_time 必须晚于 start_time")
    total_pct = sum(float(t["percent"]) for t in tges)
    if not math.isclose(total_pct, 100.0, abs_tol=1e-9):
        errors.append(f"TGE 百分比合计 {total_pct} != 100")
    if not (1 <= len(tges) <= 10):
        errors.append("TGE 期数必须在 1~10")
    if errors:
        print("[REJECTED] 参数校验失败：", file=sys.stderr)
        for e in errors:
            print("  -", e, file=sys.stderr)
        sys.exit(2)

    # TGE 时间表（输出为 DB 裸本地时间：真实时刻 - db_tz 偏移，即后端容器墙钟语义）
    db_tz = timezone(timedelta(hours=db_tz_offset_hours))
    def db_wall(dt: datetime) -> datetime:
        return dt.astimezone(db_tz).replace(tzinfo=None)

    tge_rows = []
    t = end + timedelta(minutes=tge_after_end)
    for i, tg in enumerate(tges):
        tge_rows.append((i + 1, float(tg["percent"]), t))
        t = t + timedelta(minutes=interval_min) if interval_min > 0 else t

    fmt = lambda dt: db_wall(dt).strftime("%Y-%m-%d %H:%M:%S")
    print(f"== 项目 #{project_id} Set 准备 ==")
    print(f"  DB 时区偏移  : UTC+{db_tz_offset_hours}（后端容器墙钟语义；裸值写入）")
    print(f"  支付币      : {payment_symbol} ({payment_decimals}d) {payment_address}")
    print(f"  项目币      : {token_decimals}d {token_address}")
    print(f"  价格/软/硬/单人: {token_price} / {soft_cap} / {hard_cap} / {max_per_user}")
    print(f"  raw         : price={price_raw} soft={soft_raw} hard={hard_raw} max={max_raw}")
    print(f"  时间窗(UTC) : {start:%Y-%m-%d %H:%M:%S} → {end:%Y-%m-%d %H:%M:%S}")
    for idx, pct, tt in tge_rows:
        print(f"  TGE#{idx}      : {pct}% @ {fmt(tt)}")

    # —— SQL（单事务） ——
    sql = []
    sql.append("START TRANSACTION;")
    sql.append(f"SELECT project_id, status, onchain_locked, contract_version, min_purchase_draft_amount, min_purchase_draft_token FROM projects WHERE project_id={project_id} FOR UPDATE;")
    print("  注意：若存在 Setup 最低额草稿，请通过 Admin 受控 Set 入口校验并转换；此离线 SQL 不处理该草稿。")
    # 守卫：不满足条件时中止（不更新任何行）
    guard = (
        f"UPDATE projects SET status='Set', contract_version='v2_stablecoin', chain_id={int(p.get('chain_id', 31337))},"
        f" contract_address='{p.get('ido_contract_address', '0x5FC8d32690cc91D4c39d9d3abcBD16989F875707')}',"
        f" token_address='{token_address}', token_decimals={token_decimals},"
        f" payment_token='{payment_symbol}', payment_token_address='{payment_address}',"
        f" payment_token_decimals={payment_decimals}, owner_address='{owner_address}',"
        f" fee_receiver='{p.get('fee_receiver', '0x3C44CdDdB6a900fa2b585dd299e03d12FA4293BC')}',"
        f" success_fee_rate={int(p.get('success_fee_rate', 250))},"
        f" token_price={token_price}, token_price_raw='{price_raw}',"
        f" hard_cap={hard_cap}, hard_cap_raw='{hard_raw}',"
        f" soft_cap={soft_cap}, soft_cap_raw='{soft_raw}',"
        f" max_per_user={max_per_user}, max_per_user_raw='{max_raw}',"
        f" start_time='{fmt(start)}', end_time='{fmt(end)}', tge_time='{fmt(tge_rows[0][2])}'"
        f" WHERE project_id={project_id} AND onchain_locked=0 AND min_purchase_draft_amount IS NULL AND min_purchase_amount_raw IS NULL AND status IN ('pre_launch','Set','warmup','Setup');"
    )
    sql.append(guard)
    sql.append("SET @rows = ROW_COUNT();")
    sql.append(f"DELETE FROM project_tge WHERE project_id={project_id} AND @rows=1;")
    for idx, pct, tt in tge_rows:
        sql.append(
            f"INSERT INTO project_tge (project_id,`index`,tge_percent,vesting_percent,vesting_interval,vesting_time,vesting_duration,is_notification) "
            f"SELECT {project_id},{idx},{pct:.2f},{pct:.2f},{interval_min},'{fmt(tt)}',0,1 WHERE @rows=1;"
        )
    sql.append("SELECT IF(@rows=1,'OK','GUARD_FAILED: 项目不存在/已锁定/状态不允许/存在最低额草稿或生效策略（请走 Admin Set）') AS result;")
    sql.append("COMMIT;")

    full_sql = "\n".join(sql)
    print("\n== DRY-RUN SQL (OFFLINE EVIDENCE ONLY; DO NOT EXECUTE) ==")
    print(full_sql)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
