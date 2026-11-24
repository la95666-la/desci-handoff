// Marketing FAQ is display content, not wallet eligibility or proof authority.
// Preserve every configured item; the V3 participation notice is additional.
export function projectFaqItems(raw) {
  try {
    const value = typeof raw === 'string' ? JSON.parse(raw) : raw
    return Array.isArray(value) ? value.filter(item => item && typeof item === 'object' &&
      ['q', 'q_zh', 'a', 'a_zh'].some(key => typeof item[key] === 'string' && item[key].trim())) : []
  } catch { return [] }
}

export function v3ProjectFaq(raw) {
  return [{
    q: 'Does signing up mean I can participate?',
    q_zh: '報名後就可以認購嗎？',
    a: 'Not necessarily. If this project is limited to eligible wallets, connect the wallet you plan to use. We will check whether it can participate when the sale opens.',
    a_zh: '不一定。如果項目僅限符合資格的錢包，請連接您打算使用的錢包；開放認購時，頁面會核對該錢包是否可參與。',
  }, ...projectFaqItems(raw)]
}
