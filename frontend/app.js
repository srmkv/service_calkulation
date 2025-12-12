const API_BASE = '/api';

const contentEl = document.getElementById('content');
const navItems = document.querySelectorAll('.nav-item');
const pageTitleEl = document.getElementById('page-title');
const userSwitchEl = document.getElementById('user-switch');
const avatarLetterEl = document.getElementById('avatar-letter');
const headerPlanInfoEl = document.getElementById('header-plan-info');

let currentUserId = 'admin';
// стартуем со списка калькуляторов
let currentSection = 'calculators';
// текущий калькулятор для послойного редактора
let currentLayeredCalculator = null;
// текущий калькулятор для калькулятора расстояний
let currentDistanceCalculator = null;

// кеш последнего /me
let currentMe = null;

function buildApiUrl(path) {
  const sep = path.includes('?') ? '&' : '?';
  return API_BASE + path + sep + 'as=' + encodeURIComponent(currentUserId);
}

async function fetchJSON(path) {
  const res = await fetch(buildApiUrl(path));
  if (!res.ok) {
    const err = new Error('HTTP ' + res.status);
    err.status = res.status;
    throw err;
  }
  return res.json();
}

async function postJSON(path, body) {
  const res = await fetch(buildApiUrl(path), {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  });

  if (!res.ok) {
    let message = 'HTTP ' + res.status;
    try {
      const text = await res.text();
      if (text) message = text;
    } catch (_) {}
    const err = new Error(message);
    err.status = res.status;
    throw err;
  }

  return res.json();
}

async function uploadFile(file) {
  const formData = new FormData();
  formData.append('file', file);
  const res = await fetch(buildApiUrl('/upload'), {
    method: 'POST',
    body: formData,
  });
  if (!res.ok) {
    const err = new Error('HTTP ' + res.status);
    err.status = res.status;
    throw err;
  }
  const data = await res.json();
  return data.url;
}

const CALC_TYPE_LABELS = {
  layered: 'Послойный',
  distance: 'Расчёт доставки',
  on_site: 'Выезд замерщика',
  mortgage: 'Ипотека',
};

// popup о лимите тарифа
function showPlanLimitPopup(serverMessage) {
  const existing = document.getElementById('plan-limit-modal');
  if (existing) existing.remove();

  const backdrop = document.createElement('div');
  backdrop.id = 'plan-limit-modal';
  backdrop.style.position = 'fixed';
  backdrop.style.inset = '0';
  backdrop.style.background = 'rgba(15, 23, 42, 0.45)';
  backdrop.style.display = 'flex';
  backdrop.style.alignItems = 'center';
  backdrop.style.justifyContent = 'center';
  backdrop.style.zIndex = '9999';

  const modal = document.createElement('div');
  modal.className = 'card';
  modal.style.maxWidth = '420px';
  modal.style.width = '100%';
  modal.style.margin = '16px';
  modal.style.background = '#ffffff';
  modal.style.borderRadius = '16px';
  modal.style.boxShadow = '0 20px 45px rgba(15, 23, 42, 0.25)';

  modal.innerHTML = `
    <div class="card-title">Лимит текущего тарифа</div>
    <div class="card-subtitle" style="margin-bottom:12px;">
      Вы достигли лимита по количеству калькуляторов для текущего тарифа.
    </div>
    <p class="small" style="margin-bottom:16px;color:#6b7280;">
      ${serverMessage || 'Чтобы создать больше калькуляторов, перейдите на более высокий тариф.'}
    </p>
    <div style="display:flex; justify-content:flex-end; gap:8px; margin-top:8px;">
      <button type="button" class="btn secondary" id="plan-limit-close-btn">Закрыть</button>
      <button type="button" class="btn primary" id="plan-limit-goto-billing-btn">
        Перейти к тарифам
      </button>
    </div>
  `;

  backdrop.appendChild(modal);
  document.body.appendChild(backdrop);

  const closeBtn = modal.querySelector('#plan-limit-close-btn');
  const gotoBtn = modal.querySelector('#plan-limit-goto-billing-btn');

  closeBtn.addEventListener('click', () => {
    backdrop.remove();
  });

  gotoBtn.addEventListener('click', () => {
    backdrop.remove();
    currentSection = 'billing';
    setActiveNav('billing');
    loadSection('billing');
  });

  backdrop.addEventListener('click', (e) => {
    if (e.target === backdrop) {
      backdrop.remove();
    }
  });
}

// --- работа с /me и шапкой ---

async function refreshMeAndHeader() {
  try {
    const me = await fetchJSON('/me');
    currentMe = me;
    updateHeaderFromMe(me);
  } catch (err) {
    console.error('Failed to load /me for header', err);
    if (headerPlanInfoEl) {
      headerPlanInfoEl.textContent = 'Не удалось загрузить информацию о тарифе';
    }
  }
}

function updateHeaderFromMe(me) {
  if (!headerPlanInfoEl || !me) return;

  const plan = me.plan;
  const user = me.user;

  if (!plan) {
    headerPlanInfoEl.textContent = 'Тариф не выбран';
    return;
  }

  const planName = plan.name || plan.id || 'Тариф';

  const leadsUsed = typeof me.leadsUsed === 'number' ? me.leadsUsed : 0;
  const calcsUsed = typeof me.calcsUsed === 'number' ? me.calcsUsed : 0;

  // лимит по заявкам
  const leadsLimitNum =
    typeof plan.maxLeads === 'number' && plan.maxLeads > 0
      ? plan.maxLeads
      : null;
  const leadsLimitText = leadsLimitNum ? leadsLimitNum : '∞';

  // лимит по расчётам:
  // 1) если есть plan.maxCalcs – берём его
  // 2) иначе если есть лимит по заявкам – берём 2 * maxLeads
  let calcsLimitNum = null;
  if (typeof plan.maxCalcs === 'number' && plan.maxCalcs > 0) {
    calcsLimitNum = plan.maxCalcs;
  } else if (leadsLimitNum) {
    calcsLimitNum = leadsLimitNum * 2;
  }
  const calcsLimitText = calcsLimitNum ? calcsLimitNum : '∞';

  const leadsPart = `Заявки: ${leadsUsed}/${leadsLimitText}`;
  const calcsPart = `Расчёты: ${calcsUsed}/${calcsLimitText}`;

  const statusPart =
    user && user.planActive === false ? ' · тариф не активен' : '';

  headerPlanInfoEl.textContent = `Ваш тариф: ${planName} · ${leadsPart} · ${calcsPart}${statusPart}`;
}

// --- init current user ---

function initCurrentUser() {
  const saved = window.localStorage.getItem('saasCurrentUserId');
  if (saved) {
    currentUserId = saved;
  } else {
    currentUserId = 'admin';
  }

  if (userSwitchEl) {
    userSwitchEl.value = currentUserId;
    userSwitchEl.addEventListener('change', () => {
      currentUserId = userSwitchEl.value;
      window.localStorage.setItem('saasCurrentUserId', currentUserId);
      updateAvatar();
      refreshMeAndHeader();
      loadSection(currentSection);
    });
  }

  updateAvatar();
  refreshMeAndHeader();
}

function updateAvatar() {
  let letter = 'A';
  if (currentUserId === 'admin') letter = 'A';
  if (currentUserId === 'user1') letter = '1';
  if (currentUserId === 'user2') letter = '2';
  if (avatarLetterEl) avatarLetterEl.textContent = letter;
}

// --- navigation ---

navItems.forEach((btn) => {
  btn.addEventListener('click', () => {
    const section = btn.dataset.section;
    currentSection = section;
    setActiveNav(section);
    loadSection(section);
  });
});

function setActiveNav(section) {
  navItems.forEach((btn) => {
    btn.classList.toggle('active', btn.dataset.section === section);
  });
  const titles = {
    dashboard: 'Дашборд',
    calculators: 'Калькуляторы',
    layers: 'Послойный калькулятор',
    distance: 'Калькулятор доставки',
    leads: 'Заявки',
    embeds: 'Встройка',
    integrations: 'Интеграции',
    users: 'Пользователи',
    billing: 'Биллинг',
    settings: 'Настройки',
  };
  pageTitleEl.textContent = titles[section] || 'Кабинет';
}

// --- section router ---

async function loadSection(section) {
  try {
    if (section === 'layers') {
      const cfg = await fetchJSON('/layers/config');
      renderLayersBuilder(cfg, currentLayeredCalculator);
      return;
    }

    if (section === 'billing') {
      await renderBilling();
      return;
    }

    if (section === 'users') {
      await renderAdminUsers();
      return;
    }

    if (section === 'calculators') {
      await renderCalculators();
      return;
    }
    if (section === 'distance') {
      const cfg = await fetchJSON('/distance/config');
      renderDistanceBuilder(cfg, currentDistanceCalculator);
      return;
    }
    if (section === 'settings') {
      await renderSettings();
      return;
    }
    contentEl.innerHTML = `
      <div class="card">
        <div class="card-title">Раздел "${pageTitleEl.textContent}"</div>
        <p class="card-subtitle">Функционал в разработке.</p>
      </div>
    `;
  } catch (err) {
    console.error(err);
    contentEl.innerHTML = `
      <div class="card">
        <div class="card-title">Ошибка</div>
        <p>Не удалось загрузить данные.</p>
      </div>
    `;
  }
}

// --- Settings ---

async function renderSettings() {
  contentEl.innerHTML = `
    <div class="card">
      <div class="card-title">Загрузка настроек...</div>
    </div>
  `;

  try {
    const url = buildApiUrl('/admin/settings');
    const res = await fetch(url, { method: 'GET' });

    if (res.status === 403) {
      contentEl.innerHTML = `
        <div class="card">
          <div class="card-title">Нет доступа</div>
          <p class="card-subtitle">
            Раздел "Настройки" доступен только администратору.
            Выберите пользователя "Администратор" в правом верхнем углу.
          </p>
        </div>
      `;
      return;
    }

    if (!res.ok) {
      throw new Error('HTTP ' + res.status);
    }

    const data = await res.json();

    const root = document.createElement('div');

    const card = document.createElement('div');
    card.className = 'card';
    card.innerHTML = `
      <div class="card-title">Технические настройки</div>
      <div class="card-subtitle">
        Здесь администратор может указать базовые адреса сервисов маршрутизации на основе OpenStreetMap.
      </div>

      <div class="field">
        <label class="field-label">OSRM base URL</label>
        <input type="text" id="osrm-base-url-input" placeholder="https://router.project-osrm.org" />
        <p class="small">
          Сервис построения маршрутов. По умолчанию используется публичный инстанс OSRM.
        </p>
      </div>

      <div class="field">
        <label class="field-label">Nominatim base URL</label>
        <input type="text" id="nominatim-base-url-input" placeholder="https://nominatim.openstreetmap.org" />
        <p class="small">
          Сервис геокодирования (поиск координат по адресу). По умолчанию используется публичный Nominatim.
        </p>
      </div>
      <div class="field">
        <label class="field-label">Telegram bot token</label>
       <input type="text" id="tg-bot-token-input" placeholder="123456:ABC-DEF..." />

        <p class="small">
          Токен вашего Telegram-бота из @BotFather. Используется для отправки уведомлений
          с результатами расчётов.
        </p>
      </div>

      <div class="field" style="display:flex; gap:8px; align-items:center;">
        <button class="btn primary" id="settings-save-btn" type="button">Сохранить</button>
      </div>
    `;

    root.appendChild(card);
    contentEl.innerHTML = '';
    contentEl.appendChild(root);

    const osrmInput = document.getElementById('osrm-base-url-input');
    const nominatimInput = document.getElementById('nominatim-base-url-input');
    const tgTokenInput = document.getElementById('tg-bot-token-input');
    const saveBtn = document.getElementById('settings-save-btn');

    if (data && data.osrmBaseUrl) {
      osrmInput.value = data.osrmBaseUrl;
    }
    if (data && data.nominatimBaseUrl) {
      nominatimInput.value = data.nominatimBaseUrl;
    }
    if (data && data.telegramBotToken) {
      tgTokenInput.value = data.telegramBotToken;
    }

    saveBtn.addEventListener('click', async () => {
      const osrmBaseUrl = osrmInput.value.trim();
      const nominatimBaseUrl = nominatimInput.value.trim();
      const telegramBotToken = tgTokenInput.value.trim();
      try {
        saveBtn.disabled = true;
        saveBtn.textContent = 'Сохранение...';

        const res2 = await fetch(buildApiUrl('/admin/settings'), {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({
            osrmBaseUrl,
            nominatimBaseUrl,
            telegramBotToken,
          }),
        });

        if (!res2.ok) {
          throw new Error('HTTP ' + res2.status);
        }

        await res2.json();
        alert('Настройки сохранены');
      } catch (err) {
        console.error(err);
        alert('Не удалось сохранить настройки');
      } finally {
        saveBtn.disabled = false;
        saveBtn.textContent = 'Сохранить';
      }
    });
  } catch (err) {
    console.error(err);
    contentEl.innerHTML = `
      <div class="card">
        <div class="card-title">Ошибка</div>
        <p>Не удалось загрузить настройки.</p>
      </div>
    `;
  }
}

// --- Billing / plans ---

async function renderBilling() {
  contentEl.innerHTML = `
    <div class="card">
      <div class="card-title">Загрузка тарифа...</div>
    </div>
  `;

  let me;
  let plans;

  try {
    [me, plans] = await Promise.all([
      fetchJSON('/me'),
      fetchJSON('/plans'),
    ]);
  } catch (err) {
    console.error(err);
    contentEl.innerHTML = `
      <div class="card">
        <div class="card-title">Ошибка</div>
        <p>Не удалось загрузить данные о текущем пользователе или тарифах.</p>
      </div>
    `;
    return;
  }

  const user = (me && me.user) || null;
  const currentPlan = (me && me.plan) || null;
  const leadsUsed = typeof me.leadsUsed === 'number' ? me.leadsUsed : 0;
  const calcsUsed = typeof me.calcsUsed === 'number' ? me.calcsUsed : 0;
  const userTelegramChatId =
  user && (user.telegramChatId || user.tgChatId || '');

  plans = Array.isArray(plans) ? plans : [];

  const root = document.createElement('div');

  // --- карточка "Текущий тариф" ---

  const currentCard = document.createElement('div');
  currentCard.className = 'card';
  if (!currentPlan) {
    currentCard.innerHTML = `
      <div class="card-title">Текущий тариф</div>
      <div class="card-subtitle">
        Тариф не выбран. Обратитесь к администратору или выберите тариф из списка ниже.
      </div>
    `;
  } else {
    const leadsLimit =
      typeof currentPlan.maxLeads === 'number' && currentPlan.maxLeads > 0
        ? currentPlan.maxLeads
        : null;
    const calcsLimit =
      typeof currentPlan.maxCalcs === 'number' && currentPlan.maxCalcs > 0
        ? currentPlan.maxCalcs
        : (leadsLimit ? leadsLimit * 2 : null);

    const leadsLimitText = leadsLimit ? leadsLimit : '∞';
    const calcsLimitText = calcsLimit ? calcsLimit : '∞';

    const price =
      typeof currentPlan.price === 'number'
        ? currentPlan.price.toLocaleString('ru-RU') + ' ₽/мес'
        : String(currentPlan.price || '');

    const planActive = !user || user.planActive !== false;

    currentCard.innerHTML = `
      <div class="card-title">Ваш текущий тариф: ${currentPlan.name || currentPlan.id}</div>
      <div class="card-subtitle">
        ${currentPlan.description || 'Описание тарифа отсутствует.'}
      </div>

      <div class="field" style="margin-top:8px;">
        <div class="small">
          Стоимость: <strong>${price || 'по договорённости'}</strong>
        </div>
        <div class="small">
          Калькуляторов: <strong>${currentPlan.maxCalculators}</strong>
        </div>
        <div class="small">
          Лимит заявок: <strong>${leadsUsed}/${leadsLimitText}</strong>
        </div>
        <div class="small">
          Лимит расчётов: <strong>${calcsUsed}/${calcsLimitText}</strong>
        </div>

        <div class="field" style="margin-top:12px;">
          <label class="field-label">Уведомления в Telegram</label>
          <p class="small">
            Укажите ваш Telegram ID, чтобы получать сюда копии расчётов по калькуляторам.
          </p>
          <div style="display:flex; gap:8px; align-items:center; flex-wrap:wrap;">
            <input type="text" id="billing-tg-chat-id" placeholder="Например, 123456789" style="max-width:220px;" />
            <button type="button" class="btn secondary" id="billing-tg-chat-save">
              Сохранить
            </button>
          </div>
          <p class="small" style="margin-top:4px;">
            Узнать свой ID можно через бота <code>@userinfobot</code> (команда /start).
          </p>
        </div>

        ${
          planActive
            ? ''
            : '<div class="small" style="color:#f97316;margin-top:4px;">Тариф не активен. Обратитесь к администратору или продлите подписку.</div>'
        }
      </div>
    `;
  }

  root.appendChild(currentCard);
     // --- Telegram ID в биллинге ---
  const tgChatInput = document.getElementById('billing-tg-chat-id');
  const tgChatSaveBtn = document.getElementById('billing-tg-chat-save');

  if (tgChatInput && typeof userTelegramChatId === 'string') {
    tgChatInput.value = userTelegramChatId;
  }

  if (tgChatInput && tgChatSaveBtn && user && user.id) {
    tgChatSaveBtn.addEventListener('click', async () => {
      const chatId = tgChatInput.value.trim();
      if (!chatId) {
        if (!confirm('Оставить Telegram ID пустым? Уведомления приходить не будут.')) {
          return;
        }
      }

      try {
        tgChatSaveBtn.disabled = true;
        tgChatSaveBtn.textContent = 'Сохраняем...';

        const res = await fetch(buildApiUrl('/me/telegram'), {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ telegramChatId: chatId }),
        });

        if (!res.ok) {
          const txt = await res.text();
          throw new Error(txt || ('HTTP ' + res.status));
        }

        const updatedMe = await res.json();
        currentMe = updatedMe;
        updateHeaderFromMe(updatedMe);
        alert('Telegram ID сохранён');
      } catch (err) {
        console.error(err);
        alert('Не удалось сохранить Telegram ID: ' + (err.message || err));
      } finally {
        tgChatSaveBtn.disabled = false;
        tgChatSaveBtn.textContent = 'Сохранить';
      }
    });
  }

  // --- список доступных тарифов ---

  const plansCard = document.createElement('div');
  plansCard.className = 'card';
  plansCard.innerHTML = `
    <div class="card-title">Доступные тарифы</div>
    <div class="card-subtitle">
      Тарифы загружаются из БД. Здесь можно сравнить ограничения и возможности.
      Переключение тарифа пока демонстрационное — в реальной версии будет оплата и смена плана.
    </div>
    <div class="plans-grid" id="plans-grid" style="display:grid;grid-template-columns:repeat(auto-fit,minmax(220px,1fr));gap:12px;margin-top:12px;"></div>
  `;

  const grid = plansCard.querySelector('#plans-grid');

  if (!plans.length) {
    grid.innerHTML = `<p class="small">Тарифы пока не настроены. Добавьте записи в таблицу <code>plans</code> в БД.</p>`;
  } else {
    plans.forEach((p) => {
      const isCurrent = currentPlan && currentPlan.id === p.id;

      const price =
        typeof p.price === 'number'
          ? p.price.toLocaleString('ru-RU') + ' ₽/мес'
          : String(p.price || '');

      const leadsLimit =
        typeof p.maxLeads === 'number' && p.maxLeads > 0 ? p.maxLeads : null;
      const calcsLimit =
        typeof p.maxCalcs === 'number' && p.maxCalcs > 0
          ? p.maxCalcs
          : (leadsLimit ? leadsLimit * 2 : null);

      const card = document.createElement('div');
      card.className = 'card';
      card.style.border = isCurrent ? '1px solid #4f46e5' : '1px solid #e5e7eb';
      card.style.boxShadow = isCurrent
        ? '0 0 0 1px rgba(79,70,229,0.3)'
        : 'none';

      card.innerHTML = `
        <div class="card-title" style="margin-bottom:4px;">
          ${p.name || p.id}
          ${
            isCurrent
              ? '<span class="badge" style="margin-left:6px;background:#eef2ff;color:#4f46e5;font-size:11px;padding:2px 6px;border-radius:999px;">Текущий</span>'
              : ''
          }
        </div>
        <div class="card-subtitle" style="min-height:40px;">
          ${p.description || 'Без описания'}
        </div>
        <div class="field" style="margin-top:8px;">
          <div class="small">Стоимость: <strong>${price || 'по договорённости'}</strong></div>
          <div class="small">Калькуляторов: <strong>${p.maxCalculators}</strong></div>
          <div class="small">Лимит заявок: <strong>${leadsLimit ? leadsLimit : '∞'}</strong></div>
          <div class="small">Лимит расчётов: <strong>${calcsLimit ? calcsLimit : '∞'}</strong></div>
        </div>
        <div class="field" style="margin-top:8px;">
          <button type="button" class="btn primary btn-choose-plan" data-plan-id="${p.id}" ${
            isCurrent ? 'disabled' : ''
          }>
            ${isCurrent ? 'Текущий тариф' : 'Выбрать тариф'}
          </button>
        </div>
      `;

      grid.appendChild(card);
    });
  }

  root.appendChild(plansCard);

  contentEl.innerHTML = '';
  contentEl.appendChild(root);

  // обработка Telegram chat id
  const tgIdInput = root.querySelector('#billing-tg-chat-id');
  const tgSaveBtn = root.querySelector('#billing-tg-chat-save');
  if (tgIdInput && tgSaveBtn && user) {
    if (user.telegramChatId) {
      tgIdInput.value = String(user.telegramChatId);
    }

    tgSaveBtn.addEventListener('click', async () => {
      const val = tgIdInput.value.trim();
      if (!val) {
        if (!confirm('Очистить Telegram ID?')) return;
      }

      try {
        tgSaveBtn.disabled = true;
        tgSaveBtn.textContent = 'Сохранение...';

        await fetch(buildApiUrl('/me/telegram'), {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ chatId: val }),
        });

        // локально обновим currentMe, если он есть
        if (currentMe && currentMe.user) {
          currentMe.user.telegramChatId = val || null;
        }

        alert('Telegram ID сохранён');
      } catch (err) {
        console.error(err);
        alert('Не удалось сохранить Telegram ID');
      } finally {
        tgSaveBtn.disabled = false;
        tgSaveBtn.textContent = 'Сохранить';
      }
    });
  }

  // пока делаем кнопки "демо" для смены тарифа
  root.querySelectorAll('.btn-choose-plan').forEach((btn) => {
    btn.addEventListener('click', async () => {
      const planId = btn.getAttribute('data-plan-id');
      if (!planId) return;

      if (!me || !me.user) {
        alert('Нет данных о текущем пользователе');
        return;
      }

      if (!confirm('Переключить тариф на план: ' + planId + '?')) {
        return;
      }

      const u = me.user;

      try {
        btn.disabled = true;
        btn.textContent = 'Переключаем...';

        const body = {
          name: u.name || '',
          email: u.email || '',
          role: u.role || 'user',
          planId: planId,
          planActive: true,
        };

        const res = await fetch(
          buildApiUrl('/admin/users/' + encodeURIComponent(u.id)),
          {
            method: 'PUT',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(body),
          }
        );

        if (!res.ok) {
          const txt = await res.text();
          throw new Error(txt || 'HTTP ' + res.status);
        }

        // обновляем /me и шапку
        const updatedMe = await fetchJSON('/me');
        currentMe = updatedMe;
        updateHeaderFromMe(updatedMe);

        // перерисовываем биллинг
        await renderBilling();
      } catch (err) {
        console.error(err);
        alert('Не удалось сменить тариф: ' + (err.message || err));
      }
    });
  });
}

// --- Admin users ---

async function renderAdminUsers() {
  contentEl.innerHTML = `
    <div class="card">
      <div class="card-title">Загрузка пользователей...</div>
    </div>
  `;

  let usersData = [];
  let plans = [];

  try {
    const [usersRes, me] = await Promise.all([
      fetch(buildApiUrl('/admin/users')),
      fetchJSON('/me'),
    ]);

    if (usersRes.status === 403) {
      contentEl.innerHTML = `
        <div class="card">
          <div class="card-title">Нет доступа</div>
          <p class="card-subtitle">
            Раздел "Пользователи" доступен только администратору.
            Выберите "Администратор" в правом верхнем углу.
          </p>
        </div>
      `;
      return;
    }

    if (!usersRes.ok) {
      contentEl.innerHTML = `
        <div class="card">
          <div class="card-title">Ошибка</div>
          <p>Код ответа: ${usersRes.status}</p>
        </div>
      `;
      return;
    }

    usersData = await usersRes.json();
    plans = me.plans || [];
  } catch (err) {
    console.error(err);
    contentEl.innerHTML = `
      <div class="card">
        <div class="card-title">Ошибка</div>
        <p>Не удалось загрузить данные.</p>
      </div>
    `;
    return;
  }

  const root = document.createElement('div');

  const infoCard = document.createElement('div');
  infoCard.className = 'card';
  infoCard.innerHTML = `
    <div class="card-title">Пользователи</div>
    <div class="card-subtitle">
      Администратор может управлять клиентами, их ролями и тарифами, а также сбрасывать пароли.
    </div>
  `;
  root.appendChild(infoCard);

  const tableCard = document.createElement('div');
  tableCard.className = 'card';
  tableCard.innerHTML = `
    <div class="card-title">Список пользователей</div>
    <div id="users-table-wrap"></div>
  `;
  root.appendChild(tableCard);

  const editCard = document.createElement('div');
  editCard.className = 'card';
  editCard.id = 'user-edit-card';
  editCard.style.display = 'none';
  root.appendChild(editCard);

  contentEl.innerHTML = '';
  contentEl.appendChild(root);

  let selectedUser = null;

  function roleChip(u) {
    const label = u.role === 'admin' ? 'Администратор' : 'Пользователь';
    const extra = u.role === 'admin' ? ' chip--role-admin' : ' chip--role-user';
    return `<span class="chip${extra}">${label}</span>`;
  }

  function planChip(u) {
    if (!u.plan) return '<span class="chip chip--neutral">Нет тарифа</span>';
    return `<span class="chip chip--plan">${u.plan.name} <span class="chip__tag">${u.plan.id}</span></span>`;
  }

  function planStatusChip(u) {
    if (u.planActive) {
      return '<span class="chip chip--ok">Активен</span>';
    }
    return '<span class="chip chip--warn">Не активен</span>';
  }

  function renderTable() {
    const wrap = tableCard.querySelector('#users-table-wrap');
    if (!wrap) return;

    if (!usersData || usersData.length === 0) {
      wrap.innerHTML = `<p class="small">Пользователей пока нет.</p>`;
      return;
    }

    const rows = usersData
      .slice()
      .sort((a, b) => new Date(a.createdAt) - new Date(b.createdAt))
      .map((u) => {
        const created = u.createdAt ? new Date(u.createdAt).toLocaleString('ru-RU') : '—';
        return `
          <tr data-user-id="${u.id}">
            <td>
              <div class="user-cell">
                <div class="user-avatar">${(u.name || u.email || '?').trim()[0].toUpperCase()}</div>
                <div class="user-meta">
                  <div class="user-name">${u.name || '—'}</div>
                  <div class="user-email">${u.email}</div>
                </div>
              </div>
            </td>
            <td>${roleChip(u)}</td>
            <td>${planChip(u)}</td>
            <td>${planStatusChip(u)}</td>
            <td>${created}</td>
            <td class="user-actions">
              <button class="icon-btn icon-btn-edit btn-edit-user" type="button" title="Редактировать">
                ✏
              </button>
              <button class="icon-btn icon-btn-pass btn-pass-user" type="button" title="Сменить пароль">
                🔑
              </button>
              <button class="icon-btn icon-btn-delete btn-delete-user" type="button" title="Удалить">
                🗑
              </button>
            </td>
          </tr>
        `;
      })
      .join('');

    wrap.innerHTML = `
      <table class="users-table">
        <thead>
          <tr>
            <th>Пользователь</th>
            <th>Роль</th>
            <th>Тариф</th>
            <th>Статус тарифа</th>
            <th>Создан</th>
            <th style="width:120px;">Действия</th>
          </tr>
        </thead>
        <tbody>
          ${rows}
        </tbody>
      </table>
    `;

    wrap.querySelectorAll('.btn-edit-user').forEach((btn) => {
      btn.addEventListener('click', () => {
        const tr = btn.closest('tr');
        const id = tr?.dataset.userId;
        if (!id) return;
        const u = usersData.find((x) => x.id === id);
        if (!u) return;
        selectedUser = u;
        renderEditForm();
      });
    });

    wrap.querySelectorAll('.btn-delete-user').forEach((btn) => {
      btn.addEventListener('click', async () => {
        const tr = btn.closest('tr');
        const id = tr?.dataset.userId;
        if (!id) return;

        if (!confirm('Удалить пользователя ' + id + '?')) return;

        try {
          const res = await fetch(buildApiUrl('/admin/users/' + encodeURIComponent(id)), {
            method: 'DELETE',
          });
          if (!res.ok && res.status !== 204) {
            const text = await res.text();
            alert('Не удалось удалить пользователя: ' + text);
            return;
          }
          usersData = usersData.filter((u) => u.id !== id);
          if (selectedUser && selectedUser.id === id) {
            selectedUser = null;
            editCard.style.display = 'none';
          }
          renderTable();
        } catch (err) {
          console.error(err);
          alert('Ошибка удаления пользователя');
        }
      });
    });

    wrap.querySelectorAll('.btn-pass-user').forEach((btn) => {
      btn.addEventListener('click', () => {
        const tr = btn.closest('tr');
        const id = tr?.dataset.userId;
        if (!id) return;
        const u = usersData.find((x) => x.id === id);
        if (!u) return;
        selectedUser = u;
        renderEditForm(true);
      });
    });
  }

  function renderEditForm(focusPassword = false) {
    if (!selectedUser) {
      editCard.style.display = 'none';
      editCard.innerHTML = '';
      return;
    }

    const u = selectedUser;
    const created = u.createdAt ? new Date(u.createdAt).toLocaleString('ru-RU') : '—';
    const planId = u.plan ? u.plan.id : (u.planId || '');

    const planOptions = plans
      .map(
        (p) =>
          `<option value="${p.id}" ${p.id === planId ? 'selected' : ''}>${p.name} (${p.id})</option>`
      )
      .join('');
    const roleOptions = `
      <option value="user" ${u.role === 'user' ? 'selected' : ''}>Пользователь</option>
      <option value="admin" ${u.role === 'admin' ? 'selected' : ''}>Администратор</option>
    `;

    editCard.style.display = '';
    editCard.innerHTML = `
      <div class="card-title">Редактирование пользователя</div>
      <div class="card-subtitle small">
        ID: <strong>${u.id}</strong>, создан: ${created}
      </div>

      <div class="field">
        <label class="field-label">Имя</label>
        <input type="text" id="u-edit-name" value="${u.name || ''}" />
      </div>
      <div class="field">
        <label class="field-label">E-mail</label>
        <input type="email" id="u-edit-email" value="${u.email || ''}" />
      </div>
      <div class="field">
        <label class="field-label">Роль</label>
        <select id="u-edit-role">
          ${roleOptions}
        </select>
      </div>
      <div class="field">
        <label class="field-label">Тариф</label>
        <select id="u-edit-plan">
          <option value="">— без тарифа —</option>
          ${planOptions}
        </select>
      </div>
      <div class="field">
        <label class="field-label">
          <input type="checkbox" id="u-edit-plan-active" ${u.planActive ? 'checked' : ''} />
          Тариф активен
        </label>
      </div>

      <hr class="divider" />

      <div class="field">
        <label class="field-label">Смена пароля</label>
        <div class="password-row">
          <input type="password" id="u-edit-password" placeholder="Новый пароль" />
          <button class="btn secondary btn-sm" id="u-edit-password-btn" type="button">Сменить</button>
        </div>
        <p class="small">Пароль не отображается в списке, только устанавливается.</p>
      </div>

      <div class="field" style="display:flex; flex-wrap:wrap; gap:8px;">
        <button class="btn primary btn-sm" id="u-edit-save" type="button">Сохранить изменения</button>
        <button class="btn secondary btn-sm" id="u-edit-cancel" type="button">Закрыть</button>
      </div>
    `;

    const nameInput = editCard.querySelector('#u-edit-name');
    const emailInput = editCard.querySelector('#u-edit-email');
    const roleSelect = editCard.querySelector('#u-edit-role');
    const planSelect = editCard.querySelector('#u-edit-plan');
    const planActiveCheckbox = editCard.querySelector('#u-edit-plan-active');
    const passInput = editCard.querySelector('#u-edit-password');

    const saveBtn = editCard.querySelector('#u-edit-save');
    const passBtn = editCard.querySelector('#u-edit-password-btn');
    const cancelBtn = editCard.querySelector('#u-edit-cancel');

    cancelBtn.addEventListener('click', () => {
      selectedUser = null;
      editCard.style.display = 'none';
      editCard.innerHTML = '';
    });

    saveBtn.addEventListener('click', async () => {
      try {
        const body = {
          name: nameInput.value.trim(),
          email: emailInput.value.trim(),
          role: roleSelect.value,
          planId: planSelect.value || '',
          planActive: planActiveCheckbox.checked,
        };

        const res = await fetch(
          buildApiUrl('/admin/users/' + encodeURIComponent(u.id)),
          {
            method: 'PUT',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(body),
          }
        );

        if (!res.ok) {
          const txt = await res.text();
          alert('Ошибка сохранения: ' + txt);
          return;
        }

        const updated = await res.json();
        usersData = usersData.map((item) => (item.id === updated.id ? updated : item));
        selectedUser = updated;
        renderTable();
        renderEditForm();
      } catch (err) {
        console.error(err);
        alert('Ошибка сохранения пользователя');
      }
    });

    passBtn.addEventListener('click', async () => {
      const newPass = passInput.value;
      if (!newPass) {
        alert('Введите новый пароль');
        return;
      }
      if (!confirm('Сменить пароль пользователю ' + u.id + '?')) return;

      try {
        const res = await fetch(
          buildApiUrl('/admin/users/' + encodeURIComponent(u.id) + '/password'),
          {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ password: newPass }),
          }
        );

        if (!res.ok) {
          const txt = await res.text();
          alert('Ошибка смены пароля: ' + txt);
          return;
        }

        passInput.value = '';
        alert('Пароль обновлён');
      } catch (err) {
        console.error(err);
        alert('Ошибка смены пароля');
      }
    });

    if (focusPassword && passInput) {
      passInput.focus();
    }
  }

  renderTable();
}

// --- Calculators list + create ---

async function renderCalculators() {
  contentEl.innerHTML = `
    <div class="card">
      <div class="card-title">Загрузка калькуляторов...</div>
    </div>
  `;

  let data, me;
  let planActive = true;

  try {
    [data, me] = await Promise.all([
      fetchJSON('/calculators'),
      fetchJSON('/me'),
    ]);
  } catch (err) {
    console.error(err);
    contentEl.innerHTML = `
      <div class="card">
        <div class="card-title">Ошибка</div>
        <p>Не удалось загрузить список калькуляторов или данные пользователя.</p>
      </div>
    `;
    return;
  }

  const items = (data && data.items) || [];

  const meUser = me && me.user ? me.user : null;
  planActive = !meUser || meUser.planActive !== false;

  const root = document.createElement('div');

  if (!planActive) {
    const banner = document.createElement('div');
    banner.className = 'card';
    banner.innerHTML = `
      <div class="card-title">Тариф не активен</div>
      <div class="card-subtitle">
        Срок действия вашего тарифа завершился. 
        Калькуляторы временно недоступны для редактирования.
      </div>
      <p class="small" style="margin-top:8px;">
        Активируйте тариф, чтобы снова создавать и настраивать калькуляторы.
      </p>
      <div style="margin-top:12px;">
        <button class="btn primary" id="goto-billing-from-calcs" type="button">
          Активировать тариф
        </button>
      </div>
    `;
    root.appendChild(banner);

    const gotoBtn = banner.querySelector('#goto-billing-from-calcs');
    gotoBtn.addEventListener('click', () => {
      currentSection = 'billing';
      setActiveNav('billing');
      loadSection('billing');
    });
  }

  const headerCard = document.createElement('div');
  headerCard.className = 'card';
  headerCard.innerHTML = `
    <div style="display:flex; justify-content:space-between; align-items:center; gap:8px;">
      <div>
        <div class="card-title">Ваши калькуляторы</div>
        <div class="card-subtitle">
          Управляйте существующими калькуляторами и создавайте новые.
          ${
            !planActive
              ? '<br><span class="small" style="color:#f97316;">Создание и редактирование недоступны до активации тарифа.</span>'
              : ''
          }
        </div>
      </div>
      <div>
        <button class="btn primary btn-large" id="btn-open-create-calc" type="button" ${
          !planActive ? 'disabled' : ''
        }>
          + Создать калькулятор
        </button>
      </div>
    </div>
  `;
  root.appendChild(headerCard);

  const listCard = document.createElement('div');
  listCard.className = 'card';
  listCard.innerHTML = `
    <div class="card-title">Список калькуляторов</div>
    <div id="calc-list" class="calc-list"></div>
    <div id="calc-create-panel" class="calc-create-panel" style="display:none;"></div>
  `;
  root.appendChild(listCard);

  contentEl.innerHTML = '';
  contentEl.appendChild(root);

  const listEl = listCard.querySelector('#calc-list');
  const createPanelEl = listCard.querySelector('#calc-create-panel');
  const openCreateBtn = headerCard.querySelector('#btn-open-create-calc');

  function renderList() {
    listEl.innerHTML = '';
    if (!items.length) {
      listEl.innerHTML =
        '<p class="small">У вас пока нет калькуляторов. ' +
        (planActive
          ? 'Создайте первый.'
          : 'Активируйте тариф, чтобы создать первый калькулятор.') +
        '</p>';
      return;
    }

    items
      .slice()
      .sort((a, b) => new Date(b.createdAt) - new Date(a.createdAt))
      .forEach((c) => {
        const row = document.createElement('div');
        row.className = 'calc-item';

        const typeLabel = CALC_TYPE_LABELS[c.type] || c.type;
        const statusLabel =
          c.status === 'published'
            ? '<span class="calc-status-badge">Опубликован</span>'
            : '<span class="calc-status-badge calc-status-badge--draft">Черновик</span>';

        const created = c.createdAt
          ? new Date(c.createdAt).toLocaleString('ru-RU')
          : '—';

        const calcCount =
          typeof c.calcCount === 'number' ? c.calcCount : 0;

        const publicPath =
          c.publicPath ||
          (c.publicToken && c.ownerId
            ? `/p/${c.ownerId}/${c.publicToken}`
            : '');
        const publicUrl = publicPath
          ? window.location.origin + publicPath
          : '';

        row.innerHTML = `
          <div class="calc-item-main">
            <div>
              <span class="calc-type-badge">${typeLabel}</span>
              <strong>${c.name}</strong>
            </div>
            <div class="calc-item-meta">
              ${statusLabel}
              <span style="margin-left:8px;">ID: ${c.id}</span>
              <span style="margin-left:8px;">Создан: ${created}</span>
              <span style="margin-left:8px;">Расчётов: ${calcCount}</span>
            </div>
            ${
              publicUrl
                ? `
              <div class="calc-item-link" style="margin-top:6px;">
                <span class="small" style="display:block;margin-bottom:4px;">Публичная ссылка:</span>
                <div style="display:flex; gap:6px; align-items:center;">
                  <input type="text" class="calc-link-input" value="${publicUrl}" readonly
                         style="flex:1; font-size:12px; padding:4px 6px;" />
                  <button type="button" class="btn secondary btn-copy-link" style="white-space:nowrap;">
                    Копировать
                  </button>
                </div>
              </div>
            `
                : ''
            }
          </div>
          <div class="calc-item-actions" style="display:flex; flex-direction:column; gap:4px;">
            <button class="btn secondary btn-open" type="button"${
              !planActive ? ' disabled' : ''
            }>Открыть</button>
            <button class="btn secondary btn-delete" type="button">
              Удалить
            </button>
          </div>
        `;

        if (!planActive) {
          row.style.opacity = '0.5';
        }

        const openBtn = row.querySelector('.btn-open');
        const deleteBtn = row.querySelector('.btn-delete');

        openBtn.addEventListener('click', () => {
          if (!planActive) {
            currentSection = 'billing';
            setActiveNav('billing');
            loadSection('billing');
            return;
          }

          if (c.type === 'layered') {
            currentLayeredCalculator = c;
            currentSection = 'layers';
            setActiveNav('layers');
            loadSection('layers');
          } else if (c.type === 'distance') {
            currentDistanceCalculator = c;
            currentSection = 'distance';
            setActiveNav('distance');
            loadSection('distance');
          } else {
            alert(
              'Редактор для типа "' +
                (CALC_TYPE_LABELS[c.type] || c.type) +
                '" пока в разработке.'
            );
          }
        });

        deleteBtn.addEventListener('click', async () => {
          if (!confirm(`Удалить калькулятор "${c.name}"?`)) return;

          try {
            deleteBtn.disabled = true;
            deleteBtn.textContent = 'Удаление...';

            const res = await fetch(
              buildApiUrl('/calculators?id=' + encodeURIComponent(c.id)),
              { method: 'DELETE' }
            );

            if (!res.ok) {
              let msg = 'Не удалось удалить калькулятор';
              try {
                const txt = await res.text();
                if (txt) msg += ': ' + txt;
              } catch (_) {}
              alert(msg);
              return;
            }

            const idx = items.findIndex((x) => x.id === c.id);
            if (idx !== -1) {
              items.splice(idx, 1);
            }
            renderList();
          } catch (err) {
            console.error(err);
            alert('Ошибка удаления калькулятора');
          } finally {
            deleteBtn.disabled = false;
            deleteBtn.textContent = 'Удалить';
          }
        });

        const copyBtn = row.querySelector('.btn-copy-link');
        const linkInput = row.querySelector('.calc-link-input');
        if (copyBtn && linkInput) {
          copyBtn.addEventListener('click', () => {
            linkInput.select();
            try {
              document.execCommand('copy');
              copyBtn.textContent = 'Скопировано';
              setTimeout(() => {
                copyBtn.textContent = 'Копировать';
              }, 1500);
            } catch (e) {
              console.error(e);
            }
          });
        }

        listEl.appendChild(row);
      });
  }

  renderList();

  let createPanelVisible = false;
  let selectedType = 'layered';

  function openCreatePanel() {
    if (!planActive) {
      currentSection = 'billing';
      setActiveNav('billing');
      loadSection('billing');
      return;
    }

    createPanelVisible = true;
    createPanelEl.style.display = '';
    createPanelEl.innerHTML = `
      <div class="field">
        <label class="field-label">Название калькулятора</label>
        <input type="text" id="calc-create-name" placeholder="Например, «Прицеп – послойный калькулятор»" />
      </div>
      <div class="field">
        <label class="field-label">Тип калькулятора</label>
        <div class="calc-type-buttons">
          <button type="button" class="calc-type-btn" data-type="layered">Послойный</button>
          <button type="button" class="calc-type-btn" data-type="distance">Расчёт доставки</button>
          <button type="button" class="calc-type-btn" data-type="on_site">Выезд замерщика</button>
          <button type="button" class="calc-type-btn" data-type="mortgage">Ипотека</button>
        </div>
        <p class="small">Тип влияет на логику и интерфейс конечного калькулятора.</p>
      </div>
      <div class="field">
        <button class="btn primary" id="calc-create-submit" type="button">Создать</button>
        <button class="btn secondary" id="calc-create-cancel" type="button">Отмена</button>
      </div>
    `;

    const typeButtons = createPanelEl.querySelectorAll('.calc-type-btn');
    function updateTypeButtons() {
      typeButtons.forEach((btn) => {
        btn.classList.toggle('active', btn.dataset.type === selectedType);
      });
    }
    typeButtons.forEach((btn) => {
      btn.addEventListener('click', () => {
        selectedType = btn.dataset.type;
        updateTypeButtons();
      });
    });
    updateTypeButtons();

    const submitBtn = createPanelEl.querySelector('#calc-create-submit');
    const cancelBtn = createPanelEl.querySelector('#calc-create-cancel');
    const nameInput = createPanelEl.querySelector('#calc-create-name');

    cancelBtn.addEventListener('click', () => {
      createPanelVisible = false;
      createPanelEl.style.display = 'none';
    });

    submitBtn.addEventListener('click', async () => {
      const name = nameInput.value.trim();
      if (!name) {
        alert('Введите название калькулятора');
        return;
      }
      try {
        submitBtn.disabled = true;
        submitBtn.textContent = 'Создание...';
        const created = await postJSON('/calculators', {
          name,
          type: selectedType,
        });
        items.push(created);
        renderList();
        createPanelVisible = false;
        createPanelEl.style.display = 'none';
      } catch (err) {
        console.error(err);
        if (
          err &&
          (String(err.message).toLowerCase().includes('лимита') ||
            String(err.message).toLowerCase().includes('лимит'))
        ) {
          showPlanLimitPopup(err.message);
        } else {
          alert(err.message || 'Не удалось создать калькулятор');
        }
      } finally {
        submitBtn.disabled = false;
        submitBtn.textContent = 'Создать';
      }
    });
  }

  openCreateBtn.addEventListener('click', () => {
    if (!planActive) {
      currentSection = 'billing';
      setActiveNav('billing');
      loadSection('billing');
      return;
    }

    if (createPanelVisible) {
      createPanelVisible = false;
      createPanelEl.style.display = 'none';
      return;
    }
    openCreatePanel();
  });
}

// --- Distance builder ---

function renderDistanceBuilder(cfg, calcMeta) {
  contentEl.innerHTML = '';

  // шапка, если открыто из списка калькуляторов
  if (calcMeta) {
    const infoCard = document.createElement('div');
    infoCard.className = 'card';

    const typeLabel = CALC_TYPE_LABELS[calcMeta.type] || calcMeta.type;
    const statusLabel =
      calcMeta.status === 'published'
        ? '<span class="calc-status-badge">Опубликован</span>'
        : '<span class="calc-status-badge calc-status-badge--draft">Черновик</span>';

    const created = calcMeta.createdAt
      ? new Date(calcMeta.createdAt).toLocaleString('ru-RU')
      : '—';
    const calcCount =
      typeof calcMeta.calcCount === 'number' ? calcMeta.calcCount : 0;

    const publicPath =
      calcMeta.publicPath ||
      (calcMeta.publicToken && calcMeta.ownerId
        ? `/p/${calcMeta.ownerId}/${calcMeta.publicToken}`
        : '');
    const publicUrl = publicPath ? window.location.origin + publicPath : '';

    infoCard.innerHTML = `
      <div class="card-title">${calcMeta.name || 'Калькулятор доставки'}</div>
      <div class="card-subtitle">
        Тип: ${typeLabel}. ${statusLabel}
      </div>
      <p class="small" style="margin-top:4px;">
        ID: ${calcMeta.id}, создан: ${created}, расчётов: ${calcCount}.
      </p>
      ${
        publicUrl
          ? `
        <div class="field" style="margin-top:8px;">
          <label class="field-label">Публичная ссылка</label>
          <div style="display:flex; gap:6px; align-items:center;">
            <input type="text" class="calc-link-input" value="${publicUrl}" readonly
                   style="flex:1; font-size:12px; padding:4px 6px;" />
            <button type="button" class="btn secondary" id="dist-copy-link-btn">
              Копировать
            </button>
          </div>
        </div>
      `
          : ''
      }
    `;

    contentEl.appendChild(infoCard);

    const copyBtn = infoCard.querySelector('#dist-copy-link-btn');
    const linkInput = infoCard.querySelector('.calc-link-input');
    if (copyBtn && linkInput) {
      copyBtn.addEventListener('click', () => {
        linkInput.select();
        try {
          document.execCommand('copy');
          copyBtn.textContent = 'Скопировано';
          setTimeout(() => {
            copyBtn.textContent = 'Копировать';
          }, 1500);
        } catch (e) {
          console.error(e);
        }
      });
    }
  }

  const wrapper = document.createElement('div');
  wrapper.className = 'grid grid-2';

  const left = document.createElement('div');
  const right = document.createElement('div');

  wrapper.appendChild(left);
  wrapper.appendChild(right);
  contentEl.appendChild(wrapper);

  const state = {
    basePrice: (cfg && typeof cfg.basePrice === 'number') ? cfg.basePrice : 1500,
    pricePerKm: (cfg && typeof cfg.pricePerKm === 'number') ? cfg.pricePerKm : 45,
    loadingPrice: (cfg && typeof cfg.loadingPrice === 'number') ? cfg.loadingPrice : 0,
    unloadingPrice: (cfg && typeof cfg.unloadingPrice === 'number') ? cfg.unloadingPrice : 0,
    vehicleCoefs: Object.assign(
      { small: 1.0, medium: 1.2, large: 1.5 },
      (cfg && cfg.vehicleCoefs) || {}
    ),
  };

  // --- левая колонка: настройки ---

  left.innerHTML = `
    <div class="card">
      <div class="card-title">Калькулятор доставки по расстоянию</div>
      <div class="card-subtitle">
        Настройте базовую цену, стоимость километра и доп. услуги. Эти параметры используются
        во всех калькуляторах типа «Расчёт доставки».
      </div>

      <div class="field">
        <label class="field-label">Базовая стоимость, ₽</label>
        <input type="number" id="dist-base-price" min="0" step="50" value="${state.basePrice}" />
      </div>

      <div class="field">
        <label class="field-label">Тариф за километр, ₽</label>
        <input type="number" id="dist-price-per-km" min="0" step="1" value="${state.pricePerKm}" />
      </div>

      <div class="inline" style="margin-bottom:10px;">
        <div class="field">
          <label class="field-label">Погрузка, ₽</label>
          <input type="number" id="dist-loading-price" min="0" step="50" value="${state.loadingPrice}" />
        </div>
        <div class="field">
          <label class="field-label">Разгрузка, ₽</label>
          <input type="number" id="dist-unloading-price" min="0" step="50" value="${state.unloadingPrice}" />
        </div>
      </div>

      <div class="field">
        <label class="field-label">Коэффициенты по типу транспорта</label>
        <div class="small" style="margin-bottom:4px;">Можно увеличить цену для более тяжёлых машин.</div>
        <div class="inline" style="margin-bottom:6px;">
          <div class="field">
            <label class="field-label">До 1,5 т</label>
            <input type="number" step="0.1" id="coef-small" value="${(state.vehicleCoefs.small || 1).toFixed(1)}" />
          </div>
          <div class="field">
            <label class="field-label">До 3,5 т</label>
            <input type="number" step="0.1" id="coef-medium" value="${(state.vehicleCoefs.medium || 1.2).toFixed(1)}" />
          </div>
          <div class="field">
            <label class="field-label">5 т и выше</label>
            <input type="number" step="0.1" id="coef-large" value="${(state.vehicleCoefs.large || 1.5).toFixed(1)}" />
          </div>
        </div>
      </div>

      <div class="card" style="margin-top:10px; padding-top:10px;">
        <div class="card-title">Сохранить настройки</div>
        <p class="small">
          Конфигурация хранится на сервере (мок хранилища). В рабочей версии здесь будет ваша БД.
        </p>
        <button class="btn primary" id="dist-save-btn" type="button">Сохранить конфигурацию</button>
      </div>
    </div>
  `;

  // --- правая колонка: превью калькулятора + карта ---

  right.innerHTML = `
    <div class="card">
      <div class="card-title">Превью калькулятора доставки</div>
      <div class="card-subtitle">
        Так клиент увидит калькулятор на вашем сайте. Расчёт маршрута выполняется на сервере через OpenStreetMap/OSRM.
      </div>

      <form id="dist-preview-form">
        <div class="field">
          <label class="field-label">Откуда</label>
          <input type="text" id="dist-from" placeholder="Например, Москва, Варшавское шоссе 1" />
        </div>
        <div class="field">
          <label class="field-label">Куда</label>
          <input type="text" id="dist-to" placeholder="Например, Подольск, Ленина 10" />
        </div>

        <div class="field">
          <label class="field-label">Тип транспорта</label>
          <select id="dist-vehicle">
            <option value="small">Малотоннажный до 1,5 т</option>
            <option value="medium">Грузовик до 3,5 т</option>
            <option value="large">Грузовик 5+ т</option>
          </select>
        </div>

        <div class="field">
          <label class="field-label">Используемые тарифы</label>
          <div class="small">
            База: <span id="dist-preview-base"></span>, км: <span id="dist-preview-km"></span>, погрузка/разгрузка: <span id="dist-preview-load"></span>
          </div>
        </div>

        <div class="checkbox-row">
          <input type="checkbox" id="dist-roundtrip" />
          <label for="dist-roundtrip">В обе стороны (туда-обратно)</label>
        </div>

        <div style="display:flex; gap:8px; align-items:center; margin-top:8px;">
          <button type="submit" class="btn primary">
            <span class="icon">📍</span>
            Рассчитать маршрут
          </button>
          <button type="button" id="dist-reset-btn" class="btn secondary">Сбросить</button>
        </div>
      </form>

      <div id="dist-result-box" class="result-box" style="display:none; margin-top:10px;">
        <div class="result-row">
          <div class="result-label">Расстояние (одна сторона)</div>
          <div class="result-value" id="dist-result-one">—</div>
        </div>
        <div class="result-row" id="dist-result-both-row" style="display:none;">
          <div class="result-label">Расстояние (туда-обратно)</div>
          <div class="result-value" id="dist-result-both">—</div>
        </div>
        <div class="result-row">
          <div class="result-label">Базовая стоимость</div>
          <div class="result-value" id="dist-result-base">—</div>
        </div>
        <div class="result-row">
          <div class="result-label">Оплата за км</div>
          <div class="result-value" id="dist-result-km">—</div>
        </div>
        <div class="result-row">
          <div class="result-label">Погрузка / разгрузка</div>
          <div class="result-value" id="dist-result-load">—</div>
        </div>
        <div class="result-total">
          Итого ориентировочно: <strong id="dist-result-total">—</strong>
        </div>
      </div>

      <div style="margin-top:10px;">
        <div id="distance-map" style="width:100%;height:320px;border-radius:14px;overflow:hidden;"></div>
        <div class="map-caption small" style="margin-top:4px;">
          Карта использует тайлы OpenStreetMap через Leaflet.
        </div>
      </div>

      <div id="dist-error" class="error" style="display:none;"></div>
    </div>
  `;

  // --- бинды настроек ---

  const basePriceInput = document.getElementById('dist-base-price');
  const pricePerKmInput = document.getElementById('dist-price-per-km');
  const loadingInput = document.getElementById('dist-loading-price');
  const unloadingInput = document.getElementById('dist-unloading-price');
  const coefSmallInput = document.getElementById('coef-small');
  const coefMediumInput = document.getElementById('coef-medium');
  const coefLargeInput = document.getElementById('coef-large');
  const saveBtn = document.getElementById('dist-save-btn');

  basePriceInput.addEventListener('input', () => {
    state.basePrice = Number(basePriceInput.value) || 0;
    updatePreviewTariffs();
  });
  pricePerKmInput.addEventListener('input', () => {
    state.pricePerKm = Number(pricePerKmInput.value) || 0;
    updatePreviewTariffs();
  });
  loadingInput.addEventListener('input', () => {
    state.loadingPrice = Number(loadingInput.value) || 0;
    updatePreviewTariffs();
  });
  unloadingInput.addEventListener('input', () => {
    state.unloadingPrice = Number(unloadingInput.value) || 0;
    updatePreviewTariffs();
  });

  coefSmallInput.addEventListener('input', () => {
    state.vehicleCoefs.small = Number(coefSmallInput.value) || 1;
  });
  coefMediumInput.addEventListener('input', () => {
    state.vehicleCoefs.medium = Number(coefMediumInput.value) || 1;
  });
  coefLargeInput.addEventListener('input', () => {
    state.vehicleCoefs.large = Number(coefLargeInput.value) || 1;
  });

  saveBtn.addEventListener('click', async () => {
    try {
      saveBtn.disabled = true;
      saveBtn.textContent = 'Сохранение...';

      const payload = {
        basePrice: state.basePrice,
        pricePerKm: state.pricePerKm,
        loadingPrice: state.loadingPrice,
        unloadingPrice: state.unloadingPrice,
        vehicleCoefs: state.vehicleCoefs,
      };

      await postJSON('/distance/config', payload);
      alert('Настройки калькулятора доставки сохранены');
    } catch (err) {
      console.error(err);
      alert('Ошибка сохранения настроек');
    } finally {
      saveBtn.disabled = false;
      saveBtn.textContent = 'Сохранить конфигурацию';
    }
  });

  // --- превью расчёта + карта ---

  const previewBaseEl = document.getElementById('dist-preview-base');
  const previewKmEl = document.getElementById('dist-preview-km');
  const previewLoadEl = document.getElementById('dist-preview-load');

  function formatMoney(num) {
    return Math.round(num).toLocaleString('ru-RU') + ' ₽';
  }
  function formatKm(num) {
    return (Math.round(num * 10) / 10).toLocaleString('ru-RU') + ' км';
  }

  function updatePreviewTariffs() {
    previewBaseEl.textContent = formatMoney(state.basePrice || 0);
    previewKmEl.textContent = (state.pricePerKm || 0).toLocaleString('ru-RU') + ' ₽/км';
    const loadSum = (state.loadingPrice || 0) + (state.unloadingPrice || 0);
    previewLoadEl.textContent = formatMoney(loadSum);
  }

  updatePreviewTariffs();

  const previewForm = document.getElementById('dist-preview-form');
  const fromInput = document.getElementById('dist-from');
  const toInput = document.getElementById('dist-to');
  const vehicleSelect = document.getElementById('dist-vehicle');
  const roundtripInput = document.getElementById('dist-roundtrip');
  const resetBtn = document.getElementById('dist-reset-btn');

  const resultBox = document.getElementById('dist-result-box');
  const resultOne = document.getElementById('dist-result-one');
  const resultBothRow = document.getElementById('dist-result-both-row');
  const resultBoth = document.getElementById('dist-result-both');
  const resultBase = document.getElementById('dist-result-base');
  const resultKm = document.getElementById('dist-result-km');
  const resultLoad = document.getElementById('dist-result-load');
  const resultTotal = document.getElementById('dist-result-total');
  const errorBox = document.getElementById('dist-error');

  function showError(msg) {
    errorBox.textContent = msg;
    errorBox.style.display = 'block';
  }
  function hideError() {
    errorBox.textContent = '';
    errorBox.style.display = 'none';
  }
  function hideResult() {
    resultBox.style.display = 'none';
  }

  let distanceMap = null;
  let routeLayer = null;

  function initMapIfNeeded(route) {
    if (typeof L === 'undefined') {
      console.warn('Leaflet не загружен. Проверь подключение скрипта.');
      return;
    }
    if (!distanceMap) {
      distanceMap = L.map('distance-map').setView([55.751244, 37.618423], 9);
      L.tileLayer('https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png', {
        attribution: '&copy; OpenStreetMap contributors',
      }).addTo(distanceMap);
    }

    if (!route || !route.length) {
      return;
    }

    const latlngs = route.map((p) => [p.lat, p.lon]).filter((arr) => arr[0] && arr[1]);
    if (!latlngs.length) {
      return;
    }

    if (routeLayer) {
      routeLayer.remove();
      routeLayer = null;
    }

    routeLayer = L.polyline(latlngs, { weight: 4 }).addTo(distanceMap);
    distanceMap.fitBounds(routeLayer.getBounds(), { padding: [20, 20] });
  }

  previewForm.addEventListener('submit', async (e) => {
    e.preventDefault();
    hideError();

    const from = fromInput.value.trim();
    const to = toInput.value.trim();

    if (!from || !to) {
      showError('Заполните поля «Откуда» и «Куда».');
      return;
    }

    try {
      const body = {
        from,
        to,
        vehicle: vehicleSelect.value,
        roundTrip: roundtripInput.checked,
        calculatorId: calcMeta && calcMeta.id ? calcMeta.id : '',
      };

      const res = await postJSON('/distance/calc', body);

      resultBox.style.display = 'block';
      resultOne.textContent = formatKm(res.distanceOneWayKm || 0);

      if (roundtripInput.checked) {
        resultBothRow.style.display = 'flex';
        resultBoth.textContent = formatKm(res.distanceTotalKm || 0);
      } else {
        resultBothRow.style.display = 'none';
      }

      resultBase.textContent = formatMoney(res.priceBase || 0);
      resultKm.textContent = formatMoney(res.priceKm || 0);
      resultLoad.textContent = formatMoney(res.priceLoad || 0);
      resultTotal.textContent = formatMoney(res.priceTotal || 0);

      initMapIfNeeded(res.route || []);

      refreshMeAndHeader();
    } catch (err) {
      console.error(err);
      if (err && err.message) {
        showError('Ошибка расчёта: ' + err.message);
      } else {
        showError('Не удалось рассчитать маршрут. Попробуйте ещё раз.');
      }
      hideResult();
    }
  });

  resetBtn.addEventListener('click', () => {
    fromInput.value = '';
    toInput.value = '';
    roundtripInput.checked = false;
    hideError();
    hideResult();
    if (routeLayer && distanceMap) {
      routeLayer.remove();
      routeLayer = null;
    }
  });
}

// --- Layered builder ---

function renderLayersBuilder(cfg, calcMeta) {
  contentEl.innerHTML = '';

  if (calcMeta) {
    const infoCard = document.createElement('div');
    infoCard.className = 'card';

    const typeLabel = CALC_TYPE_LABELS[calcMeta.type] || calcMeta.type;
    const statusLabel =
      calcMeta.status === 'published'
        ? '<span class="calc-status-badge">Опубликован</span>'
        : '<span class="calc-status-badge calc-status-badge--draft">Черновик</span>';

    const created = calcMeta.createdAt
      ? new Date(calcMeta.createdAt).toLocaleString('ru-RU')
      : '—';
    const calcCount =
      typeof calcMeta.calcCount === 'number' ? calcMeta.calcCount : 0;

    const publicPath =
      calcMeta.publicPath ||
      (calcMeta.publicToken && calcMeta.ownerId
        ? `/p/${calcMeta.ownerId}/${calcMeta.publicToken}`
        : '');
    const publicUrl = publicPath ? window.location.origin + publicPath : '';

    infoCard.innerHTML = `
      <div class="card-title">${calcMeta.name || 'Послойный калькулятор'}</div>
      <div class="card-subtitle">
        Тип: ${typeLabel}. ${statusLabel}
      </div>
      <p class="small" style="margin-top:4px;">
        ID: ${calcMeta.id}, создан: ${created}, расчётов: ${calcCount}.
      </p>
      ${
        publicUrl
          ? `
        <div class="field" style="margin-top:8px;">
          <label class="field-label">Публичная ссылка</label>
          <div style="display:flex; gap:6px; align-items:center;">
            <input type="text" class="calc-link-input" value="${publicUrl}" readonly
                   style="flex:1; font-size:12px; padding:4px 6px;" />
            <button type="button" class="btn secondary btn-copy-link" id="layers-copy-link-btn">
              Копировать
            </button>
          </div>
        </div>
      `
          : ''
      }
    `;

    contentEl.appendChild(infoCard);

    const copyBtn = infoCard.querySelector('#layers-copy-link-btn');
    const linkInput = infoCard.querySelector('.calc-link-input');
    if (copyBtn && linkInput) {
      copyBtn.addEventListener('click', () => {
        linkInput.select();
        try {
          document.execCommand('copy');
          copyBtn.textContent = 'Скопировано';
          setTimeout(() => {
            copyBtn.textContent = 'Копировать';
          }, 1500);
        } catch (e) {
          console.error(e);
        }
      });
    }
  }

  const wrapper = document.createElement('div');
  wrapper.className = 'grid grid-2';

  const left = document.createElement('div');

  left.innerHTML = `
    <div class="card">
      <div class="card-title">Базовые виды</div>
      <div class="card-subtitle">Нулевой слой (база) + изображения по видам.</div>

      <div class="field">
        <label class="field-label">Базовая цена</label>
        <input type="number" id="base-price-input" value="${cfg.basePrice || 0}" />
      </div>

      <div class="field">
        <label class="field-label">Описание базовой комплектации</label>
        <textarea id="base-description-input" rows="3">${cfg.baseDescription || ''}</textarea>
      </div>

      <div class="field">
        <label class="field-label">
          <input type="checkbox" id="show-rear-input" ${cfg.showRear === false ? '' : 'checked'} />
          Показывать вид сзади (rear)
        </label>
        <p class="small">Если отключить, пользователь увидит только основной вид (например, спереди).</p>
      </div>

      <div class="field">
        <label class="field-label">Изображения базовых видов</label>
        <div id="baseviews-fields"></div>
        <button class="btn secondary" id="add-view-btn" type="button">Добавить вид</button>
      </div>
    </div>

    <div class="card">
      <div class="card-title">Опции / слои</div>
      <div class="card-subtitle">Каждая опция может иметь свои картинки по видам.</div>
      <div id="options-fields"></div>
      <button class="btn secondary" id="add-option-btn" type="button">Добавить опцию</button>
    </div>

    <div class="card">
      <div class="card-title">Сохранить конфигурацию</div>
      <button class="btn primary" id="save-config-btn" type="button">Сохранить</button>
      <p class="small">Сейчас конфиг хранится в памяти сервера (мок). В бою здесь будет БД.</p>
    </div>
  `;

  const right = document.createElement('div');
  right.innerHTML = `
    <div class="card">
      <div class="card-title">Превью калькулятора</div>
      <div id="calc-preview-root"></div>
      <p class="small">Так будет выглядеть калькулятор для конечного пользователя: описание, виды и опции.</p>
    </div>
  `;

  wrapper.appendChild(left);
  wrapper.appendChild(right);
  contentEl.appendChild(wrapper);

  const state = {
    baseViews: Object.assign({}, cfg.baseViews || {}),
    options: (cfg.options || []).map((o) => ({
      id: o.id || '',
      label: o.label || '',
      price: o.price || 0,
      default: !!o.default,
      order: o.order || 0,
      layers: Object.assign({}, o.layers || {}),
    })),
    basePrice: cfg.basePrice || 0,
    baseDescription: cfg.baseDescription || '',
    showRear: cfg.showRear === false ? false : true,
  };

  const baseviewsFields = document.getElementById('baseviews-fields');
  const optionsFields = document.getElementById('options-fields');
  const previewRoot = document.getElementById('calc-preview-root');
  const basePriceInput = document.getElementById('base-price-input');
  const baseDescriptionInput = document.getElementById('base-description-input');
  const showRearInput = document.getElementById('show-rear-input');

  basePriceInput.addEventListener('input', () => {
    state.basePrice = Number(basePriceInput.value) || 0;
    renderPreview();
  });

  baseDescriptionInput.addEventListener('input', () => {
    state.baseDescription = baseDescriptionInput.value;
    renderPreview();
  });

  showRearInput.addEventListener('change', () => {
    state.showRear = showRearInput.checked;
    renderPreview();
    renderBaseViews();
    renderOptionsFields();
  });

  function getAllViewKeys() {
    return Object.keys(state.baseViews);
  }

  function getViewKeysForEditing() {
    const all = getAllViewKeys();
    if (!state.showRear) {
      return all.filter((k) => k !== 'rear');
    }
    return all;
  }

  function getActiveViewKeys() {
    return getViewKeysForEditing();
  }

  let activeView = null;
  const activeOptionIds = new Set();

  function renderBaseViews() {
    baseviewsFields.innerHTML = '';
    const keys = getViewKeysForEditing();
    if (keys.length === 0) {
      const p = document.createElement('p');
      p.className = 'small';
      p.textContent = 'Пока нет ни одного вида. Добавьте хотя бы front / rear.';
      baseviewsFields.appendChild(p);
    }

    keys.forEach((viewKey) => {
      const wrap = document.createElement('div');
      wrap.className = 'field';
      wrap.innerHTML = `
        <label class="field-label">${viewKey}</label>
        <div style="display:flex; gap:6px; align-items:center;">
          <input type="text" class="view-url-input" style="flex:1;" value="${state.baseViews[viewKey] || ''}" />
          <button class="btn secondary btn-upload" type="button">Загрузить</button>
          <input type="file" class="file-input" style="display:none;" accept="image/*" />
        </div>
      `;
      const urlInput = wrap.querySelector('.view-url-input');
      const uploadBtn = wrap.querySelector('.btn-upload');
      const fileInput = wrap.querySelector('.file-input');

      urlInput.addEventListener('input', () => {
        state.baseViews[viewKey] = urlInput.value;
        renderPreview();
      });

      uploadBtn.addEventListener('click', () => fileInput.click());
      fileInput.addEventListener('change', async () => {
        const file = fileInput.files[0];
        if (!file) return;
        try {
          const url = await uploadFile(file);
          state.baseViews[viewKey] = url;
          urlInput.value = url;
          renderPreview();
        } catch (err) {
          console.error(err);
          alert('Ошибка загрузки файла');
        }
      });

      baseviewsFields.appendChild(wrap);
    });
  }

  function renderOptionsFields() {
    optionsFields.innerHTML = '';
    const viewKeysAll = getViewKeysForEditing();
    state.options.sort((a, b) => (a.order || 0) - (b.order || 0));

    state.options.forEach((opt, idx) => {
      const wrap = document.createElement('div');
      wrap.className = 'card';
      wrap.style.marginBottom = '8px';

      wrap.innerHTML = `
        <div class="field">
          <label class="field-label">ID</label>
          <input type="text" class="opt-id" value="${opt.id}" />
        </div>
        <div class="field">
          <label class="field-label">Название</label>
          <input type="text" class="opt-label" value="${opt.label}" />
        </div>
        <div class="field">
          <label class="field-label">Цена</label>
          <input type="number" class="opt-price" value="${opt.price || 0}" />
        </div>
        <div class="field">
          <label class="field-label">Порядок</label>
          <input type="number" class="opt-order" value="${opt.order || idx + 1}" />
        </div>
        <div class="field">
          <label class="field-label">
            <input type="checkbox" class="opt-default" ${opt.default ? 'checked' : ''} />
            Включено по умолчанию
          </label>
        </div>
        <div class="field">
          <label class="field-label">Изображения по видам</label>
          <div class="small" style="margin-bottom:4px;">Для каждого view можно указать свой файл.</div>
          <div class="opt-views"></div>
        </div>
        <div class="field">
          <button class="btn secondary opt-delete-btn" type="button">Удалить опцию</button>
        </div>
      `;

      const idInput = wrap.querySelector('.opt-id');
      const labelInput = wrap.querySelector('.opt-label');
      const priceInput = wrap.querySelector('.opt-price');
      const orderInput = wrap.querySelector('.opt-order');
      const defaultCheckbox = wrap.querySelector('.opt-default');
      const viewsContainer = wrap.querySelector('.opt-views');
      const deleteBtn = wrap.querySelector('.opt-delete-btn');

      idInput.addEventListener('input', () => {
        opt.id = idInput.value;
      });
      labelInput.addEventListener('input', () => {
        opt.label = labelInput.value;
      });
      priceInput.addEventListener('input', () => {
        opt.price = Number(priceInput.value) || 0;
        renderPreview();
      });
      orderInput.addEventListener('input', () => {
        opt.order = Number(orderInput.value) || 0;
      });
      defaultCheckbox.addEventListener('change', () => {
        opt.default = defaultCheckbox.checked;
        if (opt.default && opt.id) {
          activeOptionIds.add(opt.id);
        } else {
          activeOptionIds.delete(opt.id);
        }
        renderPreview();
      });

      deleteBtn.addEventListener('click', () => {
        if (!confirm('Удалить опцию "' + (opt.label || opt.id || '') + '"?')) {
          return;
        }
        const idxInState = state.options.indexOf(opt);
        if (idxInState !== -1) {
          activeOptionIds.delete(opt.id);
          state.options.splice(idxInState, 1);
          renderOptionsFields();
          renderPreview();
        }
      });

      opt.layers = opt.layers || {};
      viewsContainer.innerHTML = '';
      viewKeysAll.forEach((viewKey) => {
        const row = document.createElement('div');
        row.className = 'field';
        const currentUrl = opt.layers[viewKey] || '';
        row.innerHTML = `
          <div class="field-label">${viewKey}</div>
          <div style="display:flex; gap:6px; align-items:center;">
            <input type="text" class="view-layer-url" style="flex:1;" value="${currentUrl}" />
            <button class="btn secondary btn-upload-view" type="button">Загрузить</button>
            <input type="file" class="file-input-view" style="display:none;" accept="image/*" />
          </div>
        `;
        const urlInput = row.querySelector('.view-layer-url');
        const uploadBtn = row.querySelector('.btn-upload-view');
        const fileInput = row.querySelector('.file-input-view');

        urlInput.addEventListener('input', () => {
          opt.layers[viewKey] = urlInput.value;
          renderPreview();
        });

        uploadBtn.addEventListener('click', () => fileInput.click());
        fileInput.addEventListener('change', async () => {
          const file = fileInput.files[0];
          if (!file) return;
          try {
            const url = await uploadFile(file);
            opt.layers[viewKey] = url;
            urlInput.value = url;
            renderPreview();
          } catch (err) {
            console.error(err);
            alert('Ошибка загрузки файла');
          }
        });

        viewsContainer.appendChild(row);
      });

      optionsFields.appendChild(wrap);
    });
  }

  function renderPreview() {
    previewRoot.innerHTML = '';

    const viewKeys = getActiveViewKeys();
    if (!activeView || !viewKeys.includes(activeView)) {
      activeView = viewKeys.includes('front') ? 'front' : (viewKeys[0] || null);
    }

    if (activeOptionIds.size === 0) {
      state.options.forEach((opt) => {
        if (opt.default && opt.id) {
          activeOptionIds.add(opt.id);
        }
      });
    }

    if (viewKeys.length === 0 || !activeView) {
      previewRoot.innerHTML = '<p class="small">Добавьте хотя бы один активный вид, чтобы увидеть превью.</p>';
      return;
    }

    const container = document.createElement('div');
    container.innerHTML = `
      <div class="field" style="margin-bottom:8px;">
        <div class="field-label">Базовая комплектация</div>
        <div class="small" id="preview-base-desc"></div>
        <div class="small" id="preview-base-price" style="margin-top:4px;"></div>
      </div>
      <div style="display:grid;grid-template-columns:minmax(0,1.4fr)minmax(0,1fr);gap:12px;align-items:flex-start;">
        <div>
          <div class="field" style="margin-bottom:6px;">
            <span class="field-label">Вид</span>
            <div id="preview-view-switch"></div>
          </div>
          <div class="view-canvas" id="preview-canvas"></div>
        </div>
        <div>
          <div class="field">
            <span class="field-label">Опции</span>
            <div id="preview-options-list"></div>
          </div>
          <div class="field">
            <span class="field-label">Итого</span>
            <div class="small" id="preview-total"></div>
          </div>
        </div>
      </div>
    `;

    previewRoot.appendChild(container);

    const baseDescEl = container.querySelector('#preview-base-desc');
    const basePriceEl = container.querySelector('#preview-base-price');
    const viewSwitchEl = container.querySelector('#preview-view-switch');
    const canvasEl = container.querySelector('#preview-canvas');
    const optionsListEl = container.querySelector('#preview-options-list');
    const totalEl = container.querySelector('#preview-total');

    baseDescEl.textContent = state.baseDescription || 'Описание не задано.';
    basePriceEl.textContent = 'База: ' + (state.basePrice || 0).toLocaleString('ru-RU') + ' ₽';

    if (viewKeys.length === 1) {
      viewSwitchEl.innerHTML = '<span class="small">' + viewKeys[0] + '</span>';
    } else {
      viewKeys.forEach((vk) => {
        const btn = document.createElement('button');
        btn.type = 'button';
        btn.className = 'btn secondary';
        if (vk === activeView) btn.style.backgroundColor = '#e5e7eb';
        btn.textContent = vk === 'front' ? 'Спереди' : (vk === 'rear' ? 'Сзади' : vk);
        btn.addEventListener('click', () => {
          activeView = vk;
          renderPreview();
        });
        viewSwitchEl.appendChild(btn);
      });
    }

    optionsListEl.innerHTML = '';
    state.options
      .slice()
      .sort((a, b) => (a.order || 0) - (b.order || 0))
      .forEach((opt) => {
        if (!opt.id) return;
        const row = document.createElement('div');
        row.className = 'field';
        const checked = activeOptionIds.has(opt.id);
        row.innerHTML = `
          <label class="small">
            <input type="checkbox" class="preview-opt-checkbox" data-opt-id="${opt.id}" ${checked ? 'checked' : ''} />
            ${opt.label || opt.id}
            <span style="color:#6b7280;">(+${(opt.price || 0).toLocaleString('ru-RU')} ₽)</span>
          </label>
        `;
        const checkbox = row.querySelector('.preview-opt-checkbox');
        checkbox.addEventListener('change', () => {
          if (checkbox.checked) {
            activeOptionIds.add(opt.id);
          } else {
            activeOptionIds.delete(opt.id);
          }
          renderPreview();
        });
        optionsListEl.appendChild(row);
      });

    const baseUrl = state.baseViews[activeView];
    canvasEl.innerHTML = '';
    if (baseUrl) {
      const baseImg = document.createElement('img');
      baseImg.src = baseUrl;
      baseImg.className = 'layer-image';
      canvasEl.appendChild(baseImg);
    }

    state.options
      .slice()
      .sort((a, b) => (a.order || 0) - (b.order || 0))
      .forEach((opt) => {
        if (!opt.id) return;
        if (!activeOptionIds.has(opt.id)) return;
        if (!opt.layers) return;
        const url = opt.layers[activeView];
        if (!url) return;
        const img = document.createElement('img');
        img.src = url;
        img.className = 'layer-image';
        canvasEl.appendChild(img);
      });

    let total = state.basePrice || 0;
    let optsSum = 0;
    state.options.forEach((opt) => {
      if (opt.id && activeOptionIds.has(opt.id)) {
        optsSum += opt.price || 0;
      }
    });
    total += optsSum;

    totalEl.textContent =
      'База: ' +
      (state.basePrice || 0).toLocaleString('ru-RU') +
      ' ₽, опции: ' +
      optsSum.toLocaleString('ru-RU') +
      ' ₽, итого: ' +
      total.toLocaleString('ru-RU') +
      ' ₽';
  }

  renderBaseViews();
  renderOptionsFields();
  renderPreview();

  document.getElementById('add-view-btn').addEventListener('click', () => {
    const key = prompt('Код вида (например, front, rear, side):');
    if (!key) return;
    if (state.baseViews[key]) {
      alert('Такой вид уже есть');
      return;
    }
    state.baseViews[key] = '';
    renderBaseViews();
    renderPreview();
  });

  document.getElementById('add-option-btn').addEventListener('click', () => {
    const nextOrder =
      state.options.length > 0
        ? Math.max.apply(
            null,
            state.options.map((o) => o.order || 0)
          ) + 1
        : 1;

    state.options.push({
      id: 'option_' + nextOrder,
      label: 'Новая опция',
      price: 0,
      default: false,
      order: nextOrder,
      layers: {},
    });

    renderOptionsFields();
    renderPreview();
  });

  document.getElementById('save-config-btn').addEventListener('click', async () => {
    try {
      const payload = {
        baseViews: state.baseViews,
        options: state.options,
        basePrice: state.basePrice,
        baseDescription: state.baseDescription,
        showRear: state.showRear,
      };
      await postJSON('/layers/config', payload);
      alert('Конфигурация сохранена');
    } catch (err) {
      console.error(err);
      alert('Ошибка сохранения конфигурации');
    }
  });
}

// --- start ---

initCurrentUser();
setActiveNav('calculators');
loadSection('calculators');
