package handlers

import (
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"strings"

	"saas-calc-backend/internal/domain"
)

// /p/{userId}/{token}
func (e *Env) HandlePublicCalculatorPage(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/p/")
	parts := strings.Split(path, "/")
	if len(parts) != 2 {
		http.NotFound(w, r)
		return
	}
	ownerID := parts[0]
	token := parts[1]

	var calc *domain.Calculator
	for _, c := range e.Calculators {
		if c.OwnerID == ownerID && c.PublicToken == token {
			calc = c
			break
		}
	}

	if calc == nil {
		http.NotFound(w, r)
		return
	}

	switch calc.Type {
	case domain.CalculatorTypeLayered:
		cfg := e.LayeredConfig
		if cfg == nil {
			cfg = domain.NewDefaultLayeredConfig()
		}

		cfgJSON, err := json.Marshal(cfg)
		if err != nil {
			http.Error(w, "failed to marshal config", http.StatusInternalServerError)
			return
		}

		renderLayeredPublic(w, calc, cfgJSON)

	case domain.CalculatorTypeDistance:
		// публичный виджет расчёта доставки
		renderDistancePublic(w, calc)

	default:
		// простая заглушка для остальных типов
		renderPublicStub(w, calc)
	}
}

// простая заглушка для других типов
func renderPublicStub(w http.ResponseWriter, calc *domain.Calculator) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, `<!DOCTYPE html>
<html lang="ru">
<head>
	<meta charset="utf-8">
	<title>%s – калькулятор</title>
	<meta name="viewport" content="width=device-width, initial-scale=1">
	<style>
		body {
			margin: 0;
			font-family: system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
			background: #f3f4f6;
			color: #111827;
		}
		.wrapper {
			min-height: 100vh;
			display: flex;
			align-items: center;
			justify-content: center;
			padding: 24px;
		}
		.card {
			background: #ffffff;
			border-radius: 16px;
			box-shadow: 0 20px 45px rgba(15, 23, 42, 0.18);
			max-width: 560px;
			width: 100%%;
			padding: 24px 24px 20px;
		}
		h1 {
			font-size: 20px;
			margin: 0 0 8px 0;
		}
		p {
			margin: 4px 0;
		}
		.badge {
			display: inline-flex;
			align-items: center;
			border-radius: 999px;
			padding: 2px 10px;
			font-size: 11px;
			background: #eef2ff;
			color: #4f46e5;
			margin-bottom: 12px;
		}
		.meta {
			font-size: 12px;
			color: #6b7280;
			margin-top: 8px;
		}
	</style>
</head>
<body>
	<div class="wrapper">
		<div class="card">
			<div class="badge">Публичная ссылка</div>
			<h1>%s</h1>
			<p>Для этого типа калькулятора публичный виджет пока не реализован.</p>
			<p class="meta">
				ID калькулятора: %s<br>
				Владелец: %s<br>
				Тип: %s
			</p>
		</div>
	</div>
</body>
</html>`,
		template.HTMLEscapeString(calc.Name),
		template.HTMLEscapeString(calc.Name),
		template.HTMLEscapeString(calc.ID),
		template.HTMLEscapeString(calc.OwnerID),
		template.HTMLEscapeString(string(calc.Type)),
	)
}

// полноценный публичный послойный калькулятор
func renderLayeredPublic(w http.ResponseWriter, calc *domain.Calculator, cfgJSON []byte) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	fmt.Fprintf(w, `<!DOCTYPE html>
<html lang="ru">
<head>
	<meta charset="utf-8">
	<title>%s – калькулятор</title>
	<meta name="viewport" content="width=device-width, initial-scale=1">
	<style>
		body {
			margin: 0;
			font-family: system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
			background: #f3f4f6;
			color: #111827;
		}
		.wrapper {
			min-height: 100vh;
			display: flex;
			align-items: center;
			justify-content: center;
			padding: 24px;
		}
		.card {
			background: #ffffff;
			border-radius: 16px;
			box-shadow: 0 20px 45px rgba(15, 23, 42, 0.18);
			max-width: 960px;
			width: 100%%;
			padding: 20px 20px 18px;
		}
		h1 {
			font-size: 20px;
			margin: 0 0 6px 0;
		}
		.subtitle {
			font-size: 13px;
			color: #6b7280;
			margin-bottom: 16px;
		}
		.badge {
			display: inline-flex;
			align-items: center;
			border-radius: 999px;
			padding: 2px 10px;
			font-size: 11px;
			background: #eef2ff;
			color: #4f46e5;
			margin-bottom: 10px;
		}
		.meta {
			font-size: 11px;
			color: #9ca3af;
			margin-top: 6px;
		}
		.layout {
			display: grid;
			grid-template-columns: 1.4fr 1fr;
			gap: 16px;
			align-items: flex-start;
		}
		.section-label {
			font-size: 12px;
			font-weight: 500;
			margin-bottom: 4px;
		}
		.view-switch {
			display: inline-flex;
			gap: 8px;
			flex-wrap: wrap;
		}
		.view-btn {
			border-radius: 999px;
			border: 1px solid #e5e7eb;
			background: #f9fafb;
			padding: 3px 10px;
			font-size: 12px;
			cursor: pointer;
		}
		.view-btn.active {
			background: #e5e7eb;
			border-color: #d1d5db;
		}
		.layer-canvas {
			position: relative;
			width: 100%%;
			border-radius: 12px;
			overflow: hidden;
			background: #f3f4f6;
			min-height: 220px;
		}
		.layer-canvas-inner {
			position: relative;
			width: 100%%;
			height: 100%%;
		}
		.layer-canvas img {
			display: block;
			width: 100%%;
			height: auto;
			position: absolute;
			top: 0;
			left: 0;
			object-fit: contain;
		}
		.options-list {
			border-radius: 10px;
			border: 1px solid #e5e7eb;
			padding: 10px 10px 8px;
			max-height: 260px;
			overflow: auto;
			background: #f9fafb;
		}
		.option-row {
			font-size: 13px;
			margin-bottom: 4px;
		}
		.option-row label {
			cursor: pointer;
		}
		.option-row span.price {
			color: #6b7280;
			font-size: 12px;
			margin-left: 4px;
		}
		.total-row {
			font-size: 14px;
			font-weight: 500;
			margin-top: 10px;
		}
		.total-row span.muted {
			font-size: 12px;
			color: #6b7280;
			font-weight: 400;
		}
	</style>
</head>
<body>
	<div class="wrapper">
		<div class="card">
			<div class="badge">Публичная ссылка</div>
			<h1>%s</h1>
			<div class="subtitle">
				Послойный калькулятор. Отметьте нужные опции и переключайтесь между видами.
			</div>
			<div class="layout">
				<div>
					<div class="section-label">Вид</div>
					<div id="view-switch" class="view-switch"></div>
					<div style="margin-top:8px;">
						<div id="canvas" class="layer-canvas">
							<div class="layer-canvas-inner" id="canvas-inner"></div>
						</div>
					</div>
					<div class="meta">
						ID калькулятора: %s
					</div>
				</div>
				<div>
					<div class="section-label">Базовая комплектация</div>
					<div id="base-desc" style="font-size:13px; color:#4b5563; margin-bottom:6px;"></div>
					<div id="base-price" style="font-size:13px; color:#111827; margin-bottom:10px;"></div>
					<div class="section-label" style="margin-bottom:4px;">Опции</div>
					<div id="options-list" class="options-list"></div>
					<div id="total-row" class="total-row"></div>
				</div>
			</div>
		</div>
	</div>

	<script>
		const CFG = %s;

		(function() {
			const cfg = CFG || {};
			const baseViews = cfg.baseViews || {};
			const options = Array.isArray(cfg.options) ? cfg.options.slice() : [];
			const showRear = cfg.showRear !== false;

			const viewKeysAll = Object.keys(baseViews || {});
			const viewKeys = showRear
				? viewKeysAll
				: viewKeysAll.filter(function(k) { return k !== 'rear'; });

			let activeView = null;
			let activeOptions = new Set();

			const viewSwitchEl = document.getElementById('view-switch');
			const canvasInnerEl = document.getElementById('canvas-inner');
			const baseDescEl = document.getElementById('base-desc');
			const basePriceEl = document.getElementById('base-price');
			const optionsListEl = document.getElementById('options-list');
			const totalRowEl = document.getElementById('total-row');

			function init() {
				if (!activeView) {
					if (viewKeys.indexOf('front') >= 0) {
						activeView = 'front';
					} else if (viewKeys.length > 0) {
						activeView = viewKeys[0];
					}
				}

				options.forEach(function(o) {
					if (o && o.id && o.default) {
						activeOptions.add(o.id);
					}
				});

				baseDescEl.textContent = cfg.baseDescription || 'Описание базовой комплектации не задано.';
				var basePrice = Number(cfg.basePrice || 0);
				basePriceEl.textContent = 'Базовая цена: ' + basePrice.toLocaleString('ru-RU') + ' ₽';

				renderViewSwitch();
				renderCanvas();
				renderOptions();
				recalcTotal();
			}

			function renderViewSwitch() {
				viewSwitchEl.innerHTML = '';

				if (!viewKeys.length || !activeView) {
					var span = document.createElement('span');
					span.textContent = 'Виды не заданы';
					span.style.fontSize = '12px';
					span.style.color = '#6b7280';
					viewSwitchEl.appendChild(span);
					return;
				}

				if (viewKeys.length === 1) {
					var span1 = document.createElement('span');
					span1.textContent = viewLabel(viewKeys[0]);
					span1.style.fontSize = '12px';
					viewSwitchEl.appendChild(span1);
					return;
				}

				viewKeys.forEach(function(vk) {
					var btn = document.createElement('button');
					btn.type = 'button';
					btn.className = 'view-btn' + (vk === activeView ? ' active' : '');
					btn.textContent = viewLabel(vk);
					btn.addEventListener('click', function() {
						activeView = vk;
						renderViewSwitch();
						renderCanvas();
					});
					viewSwitchEl.appendChild(btn);
				});
			}

			function viewLabel(key) {
				if (key === 'front') return 'Спереди';
				if (key === 'rear') return 'Сзади';
				if (key === 'side') return 'Сбоку';
				return key;
			}

			function renderCanvas() {
				canvasInnerEl.innerHTML = '';

				if (!activeView) {
					return;
				}

				var baseUrl = baseViews[activeView];
				if (baseUrl) {
					var baseImg = document.createElement('img');
					baseImg.src = baseUrl;
					canvasInnerEl.appendChild(baseImg);
				}

				options
					.slice()
					.sort(function(a, b) { return (a.order || 0) - (b.order || 0); })
					.forEach(function(o) {
						if (!o || !o.id) return;
						if (!activeOptions.has(o.id)) return;
						var layers = o.layers || {};
						var url = layers[activeView];
						if (!url) return;
						var img = document.createElement('img');
						img.src = url;
						canvasInnerEl.appendChild(img);
					});
			}

			function renderOptions() {
				optionsListEl.innerHTML = '';

				if (!options.length) {
					var p = document.createElement('p');
					p.textContent = 'Опции не заданы.';
					p.style.fontSize = '12px';
					p.style.color = '#6b7280';
					optionsListEl.appendChild(p);
					return;
				}

				options
					.slice()
					.sort(function(a, b) { return (a.order || 0) - (b.order || 0); })
					.forEach(function(o) {
						if (!o || !o.id) return;
						var row = document.createElement('div');
						row.className = 'option-row';

						var id = o.id;
						var label = o.label || id;
						var price = Number(o.price || 0);
						var checked = activeOptions.has(id);

						var html =
							'<label>' +
								'<input type="checkbox" ' + (checked ? 'checked' : '') + ' data-id="' + escapeHtml(id) + '"/>' +
								' ' + escapeHtml(label) +
								' <span class="price">(+' + price.toLocaleString('ru-RU') + ' ₽)</span>' +
							'</label>';

						row.innerHTML = html;

						var input = row.querySelector('input[type="checkbox"]');
						input.addEventListener('change', function() {
							var optId = input.getAttribute('data-id');
							if (input.checked) {
								activeOptions.add(optId);
							} else {
								activeOptions.delete(optId);
							}
							renderCanvas();
							recalcTotal();
						});

						optionsListEl.appendChild(row);
					});
			}

			function recalcTotal() {
				var basePrice = Number(cfg.basePrice || 0);
				var optsSum = 0;

				options.forEach(function(o) {
					if (!o || !o.id) return;
					if (activeOptions.has(o.id)) {
						optsSum += Number(o.price || 0);
					}
				});

				var total = basePrice + optsSum;

				totalRowEl.innerHTML =
					'Итого: ' + total.toLocaleString('ru-RU') + ' ₽ ' +
					'<span class="muted">(база ' + basePrice.toLocaleString('ru-RU') +
					' ₽ + опции ' + optsSum.toLocaleString('ru-RU') + ' ₽)</span>';
			}

			function escapeHtml(str) {
				return String(str)
					.replace(/&/g, '&amp;')
					.replace(/</g, '&lt;')
					.replace(/>/g, '&gt;')
					.replace(/"/g, '&quot;')
					.replace(/'/g, '&#39;');
			}

			init();
		})();
	</script>
</body>
</html>`,
		template.HTMLEscapeString(calc.Name),
		template.HTMLEscapeString(calc.Name),
		template.HTMLEscapeString(calc.ID),
		string(cfgJSON),
	)
}

// публичный виджет для калькулятора доставки (distance)
func renderDistancePublic(w http.ResponseWriter, calc *domain.Calculator) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	name := calc.Name
	if strings.TrimSpace(name) == "" {
		name = "Калькулятор доставки"
	}
	escName := template.HTMLEscapeString(name)
	idHTML := template.HTMLEscapeString(calc.ID)
	idJS := template.JSEscapeString(calc.ID)

	fmt.Fprintf(w, `<!doctype html>
<html lang="ru">
<head>
  <meta charset="utf-8" />
  <title>%s – калькулятор</title>
  <meta name="viewport" content="width=device-width, initial-scale=1" />

  <link
    rel="stylesheet"
    href="https://unpkg.com/leaflet@1.9.4/dist/leaflet.css"
    integrity="sha256-p4NxAoJBhIIN+hmNHrzRCf9tD/miZyoHS5obTRR9BMY="
    crossorigin=""
  />

  <style>
    * { box-sizing: border-box; }

    body {
      margin: 0;
      padding: 16px;
      font-family: system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
      background: #f3f4f6;
      color: #111827;
    }

    .widget-root {
      max-width: 960px;
      margin: 0 auto;
    }

    .card {
      background: #ffffff;
      border-radius: 16px;
      padding: 16px 18px;
      box-shadow: 0 10px 30px rgba(15,23,42,0.15);
      margin-bottom: 16px;
    }

    .card-title {
      font-size: 18px;
      font-weight: 600;
      margin-bottom: 4px;
    }

    .card-subtitle {
      font-size: 13px;
      color: #6b7280;
      margin-bottom: 10px;
    }

    .badge {
      display: inline-flex;
      align-items: center;
      border-radius: 999px;
      padding: 2px 10px;
      font-size: 11px;
      background: #eef2ff;
      color: #4f46e5;
      margin-bottom: 8px;
    }

    .meta {
      font-size: 11px;
      color: #9ca3af;
      margin-top: 6px;
    }

    .field {
      margin-bottom: 10px;
    }

    .field-label {
      display: block;
      font-size: 13px;
      margin-bottom: 4px;
    }

    input[type="text"],
    input[type="number"],
    select {
      width: 100%%;
      padding: 8px 10px;
      border-radius: 10px;
      border: 1px solid #d1d5db;
      font-size: 14px;
      outline: none;
    }
    input:focus, select:focus {
      border-color: #6366f1;
      box-shadow: 0 0 0 1px rgba(99,102,241,0.3);
    }

    .checkbox-row {
      display: flex;
      align-items: center;
      gap: 6px;
      font-size: 13px;
    }

    .btn {
      border-radius: 999px;
      border: none;
      padding: 8px 16px;
      font-size: 14px;
      cursor: pointer;
      display: inline-flex;
      align-items: center;
      gap: 6px;
    }
    .btn-primary {
      background: #4f46e5;
      color: white;
    }
    .btn-primary:hover {
      background: #4338ca;
    }
    .btn-secondary {
      background: #e5e7eb;
      color: #111827;
    }

    .result-box {
      border-radius: 12px;
      background: #f9fafb;
      padding: 10px 12px;
      margin-top: 10px;
      font-size: 14px;
    }

    .result-row {
      display: flex;
      justify-content: space-between;
      font-size: 13px;
      margin-bottom: 4px;
    }
    .result-label {
      color: #6b7280;
    }
    .result-value {
      font-weight: 500;
    }
    .result-total {
      margin-top: 6px;
      font-size: 15px;
      font-weight: 600;
    }

    .error-box {
      margin-top: 8px;
      padding: 8px 10px;
      border-radius: 10px;
      background: #fee2e2;
      color: #b91c1c;
      font-size: 13px;
      display: none;
    }

    #distance-map {
      width: 100%%;
      height: 320px;
      margin-top: 10px;
      border-radius: 14px;
      overflow: hidden;
    }

    .map-caption {
      font-size: 12px;
      color: #9ca3af;
      margin-top: 4px;
    }
  </style>
</head>
<body>
  <div class="widget-root">
    <div class="card">
      <div class="badge">Публичная ссылка</div>
      <div class="card-title">%s</div>
      <div class="card-subtitle">
        Калькулятор ориентировочной стоимости доставки по адресу и расстоянию.
      </div>

      <form id="dist-form">
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

        <div class="checkbox-row" style="margin: 8px 0;">
          <input type="checkbox" id="dist-roundtrip" />
          <label for="dist-roundtrip">В обе стороны (туда-обратно)</label>
        </div>

        <div style="display:flex; gap:8px; align-items:center; margin-top:8px;">
          <button type="submit" class="btn btn-primary">
            <span>📍</span>
            <span>Рассчитать маршрут</span>
          </button>
          <button type="button" id="dist-reset" class="btn btn-secondary">Сбросить</button>
        </div>
      </form>

      <div class="meta">
        ID калькулятора: %s
      </div>

      <div id="dist-error" class="error-box"></div>

      <div id="dist-result" class="result-box" style="display:none;">
        <div class="result-row">
          <div class="result-label">Расстояние (одна сторона)</div>
          <div class="result-value" id="dist-one">—</div>
        </div>
        <div class="result-row" id="dist-both-row" style="display:none;">
          <div class="result-label">Расстояние (туда-обратно)</div>
          <div class="result-value" id="dist-both">—</div>
        </div>
        <div class="result-row">
          <div class="result-label">База</div>
          <div class="result-value" id="dist-base">—</div>
        </div>
        <div class="result-row">
          <div class="result-label">Оплата за км</div>
          <div class="result-value" id="dist-km">—</div>
        </div>
        <div class="result-row">
          <div class="result-label">Погрузка / разгрузка</div>
          <div class="result-value" id="dist-load">—</div>
        </div>
        <div class="result-total">
          Итого ориентировочно: <span id="dist-total">—</span>
        </div>
      </div>

      <div id="distance-map"></div>
      <div class="map-caption">Маршрут и карта — на базе OpenStreetMap / Leaflet.</div>
    </div>
  </div>

  <script
    src="https://unpkg.com/leaflet@1.9.4/dist/leaflet.js"
    integrity="sha256-20nQCchB9co0qIjJZRGuk2/Z9VM+kNiyxNV1lvTlZBo="
    crossorigin=""
  ></script>

  <script>
    (function() {
      const calculatorId = %q;

      function formatMoney(num) {
        return Math.round(num).toLocaleString('ru-RU') + ' ₽';
      }
      function formatKm(num) {
        return (Math.round(num * 10) / 10).toLocaleString('ru-RU') + ' км';
      }

      let map = null;
      let routeLayer = null;

      function ensureMap() {
        if (!window.L) {
          console.warn('Leaflet не загружен');
          return null;
        }
        if (!map) {
          map = L.map('distance-map').setView([55.751244, 37.618423], 9);
          L.tileLayer('https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png', {
            attribution: '&copy; OpenStreetMap contributors',
          }).addTo(map);
        }
        return map;
      }

      function drawRoute(route) {
        const m = ensureMap();
        if (!m || !route || !route.length) return;

        const latlngs = route
          .map(function(p) { return [p.lat, p.lon]; })
          .filter(function(arr) { return arr[0] && arr[1]; });

        if (!latlngs.length) return;

        if (routeLayer) {
          routeLayer.remove();
          routeLayer = null;
        }

        routeLayer = L.polyline(latlngs, { weight: 4 }).addTo(m);
        m.fitBounds(routeLayer.getBounds(), { padding: [20, 20] });
      }

      document.addEventListener('DOMContentLoaded', function() {
        const form = document.getElementById('dist-form');
        const fromInput = document.getElementById('dist-from');
        const toInput = document.getElementById('dist-to');
        const vehicleSelect = document.getElementById('dist-vehicle');
        const roundtripInput = document.getElementById('dist-roundtrip');
        const resetBtn = document.getElementById('dist-reset');

        const errorBox = document.getElementById('dist-error');
        const resultBox = document.getElementById('dist-result');
        const oneEl = document.getElementById('dist-one');
        const bothRow = document.getElementById('dist-both-row');
        const bothEl = document.getElementById('dist-both');
        const baseEl = document.getElementById('dist-base');
        const kmEl = document.getElementById('dist-km');
        const loadEl = document.getElementById('dist-load');
        const totalEl = document.getElementById('dist-total');

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

        form.addEventListener('submit', async function(e) {
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
              from: from,
              to: to,
              vehicle: vehicleSelect.value,
              roundTrip: roundtripInput.checked,
              calculatorId: calculatorId
            };

            const res = await fetch('/api/distance/calc', {
              method: 'POST',
              headers: { 'Content-Type': 'application/json' },
              body: JSON.stringify(body)
            });

            if (!res.ok) {
              const text = await res.text();
              showError('Ошибка расчёта: ' + (text || ('HTTP ' + res.status)));
              hideResult();
              return;
            }

            const data = await res.json();

            resultBox.style.display = 'block';
            oneEl.textContent = formatKm(data.distanceOneWayKm || 0);

            if (roundtripInput.checked) {
              bothRow.style.display = 'flex';
              bothEl.textContent = formatKm(data.distanceTotalKm || 0);
            } else {
              bothRow.style.display = 'none';
            }

            baseEl.textContent  = formatMoney(data.priceBase || 0);
            kmEl.textContent    = formatMoney(data.priceKm || 0);
            loadEl.textContent  = formatMoney(data.priceLoad || 0);
            totalEl.textContent = formatMoney(data.priceTotal || 0);

            drawRoute(data.route || []);
          } catch (err) {
            console.error(err);
            showError('Не удалось рассчитать маршрут. Попробуйте ещё раз.');
            hideResult();
          }
        });

        resetBtn.addEventListener('click', function() {
          fromInput.value = '';
          toInput.value = '';
          roundtripInput.checked = false;
          hideError();
          hideResult();
          if (routeLayer && map) {
            routeLayer.remove();
            routeLayer = null;
          }
        });
      });
    })();
  </script>
</body>
</html>`,
		escName,
		escName,
		idHTML,
		idJS,
	)
}
