const startButton = document.getElementById('startButton');
const stopButton = document.getElementById('stopButton');
const statusDiv = document.getElementById('status');
const logOutput = document.getElementById('log-output');
const connectorSelect = document.getElementById('connector');
const connectorParamsDiv = document.getElementById('connector-params');
const strategySelect = document.getElementById('strategy');
const strategyParamsDiv = document.getElementById('strategy-params');
const chartCanvas = document.getElementById('priceChart');
const docTabContent = document.getElementById('tab-documentation');
const modCodeTextarea = document.getElementById('mod-code');
const applyModButton = document.getElementById('applyModButton');
const validateModButton = document.getElementById('validateModButton');
const symbolSelect = document.getElementById('symbol');
const newSymbolInput = document.getElementById('new-symbol');
const addSymbolBtn = document.getElementById('add-symbol-btn');
const paperTradingToggle = document.getElementById('paper-trading');
const tickIntervalInput = document.getElementById('tick-interval');
const tickValueSpan = document.getElementById('tick-value');
const riskLevelSelect = document.getElementById('risk-level');
const clearLogsBtn = document.getElementById('clearLogsBtn');
const exportLogsBtn = document.getElementById('exportLogsBtn');
const strategyDescription = document.getElementById('strategy-description');
const modStatusDiv = document.getElementById('mod-status');
const resetZoomBtn = document.getElementById('resetZoomBtn');
const saveConfigBtn = document.getElementById('saveConfigBtn');
const loadConfigInput = document.getElementById('loadConfigInput');
const themeToggle = document.getElementById('theme-toggle');

// Performance stats elements
const totalTradesEl = document.getElementById('total-trades');
const winRateEl = document.getElementById('win-rate');
const currentPriceEl = document.getElementById('current-price');
const profitLossEl = document.getElementById('profit-loss');
const uptimeEl = document.getElementById('uptime');
const lastSignalEl = document.getElementById('last-signal');

let priceChart;
let originalGoCode;
let logs = [];

const connectorDefinitions = {
    "simulation": { 
        name: "Simulation", 
        params: {}, 
        description: "Generates realistic price movements with trends and volatility for testing strategies without real money." 
    },
    "coinbase": { 
        name: "Coinbase", 
        params: { 
            "apiKey": { label: "API Key", type: "password" }, 
            "apiSecret": { label: "API Secret", type: "password" }, 
            "secretPhrase": { label: "Secret Phrase", type: "password" }
        }, 
        description: "Connects to Coinbase for live price data and executes real trades. Requires API keys with trading permissions." 
    },
    "binance": { 
        name: "Binance", 
        params: { 
            "apiKey": { label: "API Key", type: "password" }, 
            "apiSecret": { label: "API Secret", type: "password" }
        }, 
        description: "Connects to Binance for live price data and executes real trades. Requires API keys with trading permissions." 
    }
};

const strategyDefinitions = {
    "sma_crossover": { 
        name: "SMA Crossover", 
        params: { 
            "sma_short_period": { label: "Short Period", value: 10, type: "number", min: 2, max: 50 }, 
            "sma_long_period": { label: "Long Period", value: 25, type: "number", min: 5, max: 100 }
        }, 
        description: "Generates buy signals when the short SMA crosses above the long SMA, and sell signals when it crosses below. Best for trending markets." 
    },
    "rsi_basic": { 
        name: "RSI Basic", 
        params: { 
            "rsi_period": { label: "RSI Period", value: 14, type: "number", min: 5, max: 30 }, 
            "rsi_overbought": { label: "Overbought Level", value: 70, type: "number", min: 60, max: 90 }, 
            "rsi_oversold": { label: "Oversold Level", value: 30, type: "number", min: 10, max: 40 }
        }, 
        description: "Uses the Relative Strength Index to identify overbought and oversold conditions. Good for range-bound markets." 
    },
    "stochastic": { 
        name: "Stochastic Oscillator", 
        params: { 
            "period": { label: "Period", value: 14, type: "number", min: 5, max: 50 }, 
            "overbought": { label: "Overbought", value: 80, type: "number", min: 70, max: 95 }, 
            "oversold": { label: "Oversold", value: 20, type: "number", min: 5, max: 30 }
        }, 
        description: "Measures the position of current price relative to its range over a specified period. Sensitive to market momentum." 
    },
    "bollinger": { 
        name: "Bollinger Bands", 
        params: { 
            "period": { label: "Period", value: 20, type: "number", min: 10, max: 50 }, 
            "std_dev": { label: "Std. Deviations", value: 2, type: "number", min: 1, max: 3 }
        }, 
        description: "Triggers trades when the price touches the upper or lower bands. Effective in volatile markets with mean reversion." 
    }
};

function initializeTabs() {
    const tabLinks = document.querySelectorAll('.tab-link');
    const tabContents = document.querySelectorAll('.tab-content');
    
    tabLinks.forEach(link => {
        link.addEventListener('click', (e) => {
            e.preventDefault();
            const tabId = link.dataset.tab;
            
            tabLinks.forEach(l => l.classList.remove('active'));
            tabContents.forEach(c => c.classList.remove('active'));
            
            link.classList.add('active');
            const targetContent = document.getElementById(`tab-${tabId}`);
            if (targetContent) {
                targetContent.classList.add('active');
            }
        });
    });
}

function goLog(level, message) {
    const logEntry = document.createElement('div');
    const timestamp = new Date().toLocaleTimeString();
    const logData = { timestamp, level, message };
    logs.push(logData);
    
    logEntry.innerHTML = `<span class="text-slate-500 mr-2">${timestamp}</span> <span class="font-medium">${message}</span>`;
    logEntry.classList.add('log-entry', `log-${level}`, 'pl-3', 'py-2', 'rounded-lg');
    logOutput.appendChild(logEntry);
    logOutput.scrollTop = logOutput.scrollHeight;
    
    if (logOutput.children.length > 200) {
        logOutput.removeChild(logOutput.firstChild);
    }
}

function goUpdateStatus(newStatus) {
    statusDiv.textContent = `STATUS: ${newStatus}`;
    statusDiv.className = `text-center text-sm font-medium p-3 rounded-lg text-white`;
    if (newStatus.includes('RUNNING')) { 
        statusDiv.classList.add('status-running'); 
    } else if (newStatus.includes('STOPPED')) { 
        statusDiv.classList.add('status-stopped'); 
    } else { 
        statusDiv.classList.add('status-idle'); 
    }
}

function goUpdateChart(newPrice) {
    if (!priceChart) return;
    const now = Date.now();
    priceChart.data.datasets[0].data.push({x: now, y: newPrice});
    
    // Prune old data points to keep the chart responsive and prevent memory leaks
    const maxDataPoints = 2000;
    if (priceChart.data.datasets[0].data.length > maxDataPoints) {
        priceChart.data.datasets[0].data.shift();
    }

    // Prune signal/indicator points that are older than the first visible price point
    if (priceChart.data.datasets[0].data.length > 0) {
        const timeThreshold = priceChart.data.datasets[0].data[0].x;
        // Loop through datasets from index 1 onwards (skip price)
        for (let i = 1; i < priceChart.data.datasets.length; i++) {
            const d = priceChart.data.datasets[i].data;
            while(d.length > 0 && d[0].x < timeThreshold) {
                d.shift();
            }
        }
    }
    
    priceChart.update('none');
}

function goPlotSignal(signalType, price) {
    if (!priceChart) return;
    const now = Date.now();
    const datasetIndex = signalType === 'BUY' ? 1 : 2;
    priceChart.data.datasets[datasetIndex].data.push({x: now, y: price});
    priceChart.update('none');
}

function goUpdateIndicators(jsonData) {
    if (!priceChart) return;
    try {
        const indicators = JSON.parse(jsonData);
        const now = Date.now();

        const indicatorMapping = {
            "sma_short": "SMA Short",
            "sma_long": "SMA Long",
            "bollinger_upper": "Bollinger Upper",
            "bollinger_lower": "Bollinger Lower"
        };

        const datasetMap = new Map();
        priceChart.data.datasets.forEach(ds => {
            datasetMap.set(ds.label, ds);
        });

        for (const [key, value] of Object.entries(indicators)) {
            const label = indicatorMapping[key];
            if (label) {
                const dataset = datasetMap.get(label);
                if (dataset && value) { // Ensure value is not 0 or null
                    dataset.data.push({ x: now, y: value });
                }
            }
        }
    } catch (e) {
        // Silently fail on parse error
    }
}

function goUpdatePerformanceStats(trades, winRate, currentPrice, profitLoss) {
    totalTradesEl.textContent = trades;
    winRateEl.textContent = `${winRate.toFixed(1)}%`;
    currentPriceEl.textContent = `${currentPrice.toFixed(2)}`;
    profitLossEl.textContent = `${profitLoss.toFixed(2)}`;
    profitLossEl.style.color = profitLoss >= 0 ? '#10b981' : '#ef4444';
}

function goUpdateUptime(uptimeString) {
    uptimeEl.textContent = uptimeString.split('.')[0]; // Remove milliseconds
}

function goUpdateLastSignal(signal) {
    lastSignalEl.textContent = signal;
    lastSignalEl.style.color = signal === 'BUY' ? '#10b981' : signal === 'SELL' ? '#ef4444' : '#94a3b8';
}

// CORRECTED: Fixed the entire function which was broken by a syntax error
function createParamUI(container, definitions, definitionKey) {
    container.innerHTML = '';
    const definition = definitions[definitionKey];
    if (!definition) return;
    
    if (Object.keys(definition.params).length === 0) { 
        container.innerHTML = `<p class="text-xs text-slate-500">No additional settings needed.</p>`; 
        return; 
    }
    
    for (const [key, param] of Object.entries(definition.params)) {
        const paramGroup = document.createElement('div');
        paramGroup.innerHTML = `
            <label for="param-${key}" class="block text-sm font-medium text-slate-300 mb-2">${param.label}</label>
            <input type="${param.type}" id="param-${key}" value="${param.value || ''}" 
                   class="param-input" data-param-key="${key}" 
                   ${param.min ? `min="${param.min}"` : ''} 
                   ${param.max ? `max="${param.max}"` : ''}>
            ${param.description ? `<p class="mt-1 text-xs text-slate-500">${param.description}</p>` : ''}
        `;
        container.appendChild(paramGroup);
    }
}

function updateStrategyDescription() {
    const selectedStrategy = strategySelect.value;
    const definition = strategyDefinitions[selectedStrategy];
    if (definition) {
        strategyDescription.textContent = definition.description;
    }
}

function applyConfig(config) {
    // General Settings
    if (config.symbol) {
        // Check if the symbol exists in the dropdown, if not, add it.
        let symbolOption = symbolSelect.querySelector(`option[value="${config.symbol}"]`);
        if (!symbolOption) {
            symbolOption = new Option(config.symbol, config.symbol);
            symbolSelect.add(symbolOption);
        }
        symbolSelect.value = config.symbol;
    }
    if (config.paperTrading !== undefined) {
        paperTradingToggle.checked = config.paperTrading;
    }
    if (config.tickIntervalSeconds) {
        tickIntervalInput.value = config.tickIntervalSeconds;
        tickValueSpan.textContent = `${config.tickIntervalSeconds}s`;
    }
    if (config.riskLevel) {
        riskLevelSelect.value = config.riskLevel;
    }

    // Connector
    if (config.connector) {
        connectorSelect.value = config.connector;
        // Trigger change to build the UI for connector params
        connectorSelect.dispatchEvent(new Event('change'));
        // Now that UI is built, set the values
        if (config.connectorParams) {
            for (const [key, value] of Object.entries(config.connectorParams)) {
                const input = document.getElementById(`param-${key}`);
                if (input) input.value = value;
            }
        }
    }

    // Strategy
    if (config.strategy) {
        strategySelect.value = config.strategy;
        strategySelect.dispatchEvent(new Event('change'));
        if (config.strategyParams) {
            for (const [key, value] of Object.entries(config.strategyParams)) {
                const input = document.getElementById(`param-${key}`);
                if (input) input.value = value;
            }
        }
    }
}

function validateConfig() { 
    const symbol = symbolSelect.value;
    const tickInterval = parseInt(tickIntervalInput.value, 10);
    
    if (!symbol) {
        goLog('error', 'Please select or add a trading pair in Settings.');
        return false;
    }
    
    if (tickInterval < 1 || tickInterval > 60) {
        goLog('error', 'Tick interval must be between 1 and 60 seconds.');
        return false;
    }
    
    return true; 
}

function generateFullConfig() {
    const connectorParams = {};
    document.querySelectorAll('#connector-params input').forEach(input => { 
        connectorParams[input.dataset.paramKey] = input.value; 
    });
    
    const strategyParams = {};
    document.querySelectorAll('#strategy-params input').forEach(input => { 
        strategyParams[input.dataset.paramKey] = parseFloat(input.value); 
    });
    
    return JSON.stringify({
        symbol: symbolSelect.value.toUpperCase(), 
        tickIntervalSeconds: parseInt(tickIntervalInput.value, 10), 
        paperTrading: paperTradingToggle.checked,
        connector: connectorSelect.value, 
        connectorParams: connectorParams,
        strategy: strategySelect.value, 
        strategyParams: strategyParams,
        riskLevel: riskLevelSelect.value
    }, null, 4);
}

function initializeChart() {
    const ctx = chartCanvas.getContext('2d');
    priceChart = new Chart(ctx, { 
        type: 'line', 
        data: { 
            datasets: [ 
                { 
                    label: 'Price (USD)', 
                    data: [], // Data will be {x, y} objects
                    borderColor: '#6366f1', 
                    backgroundColor: 'rgba(99, 102, 241, 0.1)', 
                    borderWidth: 2, 
                    tension: 0.1, 
                    fill: true, 
                    pointRadius: 0, 
                }, 
                { 
                    label: 'Buy Signal', 
                    data: [], 
                    type: 'scatter', 
                    backgroundColor: '#10b981', 
                    pointStyle: 'triangle', 
                    radius: 8, 
                    rotation: 0, 
                }, 
                { 
                    label: 'Sell Signal', 
                    data: [], 
                    type: 'scatter', 
                    backgroundColor: '#ef4444', 
                    pointStyle: 'triangle', 
                    radius: 8, 
                    rotation: 180, 
                },
                {
                    label: 'SMA Short',
                    data: [],
                    borderColor: '#f59e0b', // Amber
                    borderWidth: 1,
                    tension: 0.2,
                    pointRadius: 0,
                    hidden: true, // Hidden by default
                },
                {
                    label: 'SMA Long',
                    data: [],
                    borderColor: '#06b6d4', // Cyan
                    borderWidth: 1,
                    tension: 0.2,
                    pointRadius: 0,
                    hidden: true,
                },
                {
                    label: 'Bollinger Upper',
                    data: [],
                    borderColor: '#a5b4fc', // Indigo-300
                    borderDash: [5, 5], // Dashed line
                    borderWidth: 1,
                    pointRadius: 0,
                    fill: false,
                    hidden: true,
                },
                {
                    label: 'Bollinger Lower',
                    data: [],
                    borderColor: '#a5b4fc', // Indigo-300
                    borderDash: [5, 5],
                    borderWidth: 1,
                    pointRadius: 0,
                    fill: '+1', // Fill to the upper band dataset
                    backgroundColor: 'rgba(99, 102, 241, 0.05)',
                    hidden: true,
                } 
            ] 
        }, 
        options: { 
            responsive: true, 
            maintainAspectRatio: false, 
            scales: { 
                x: {
                    type: 'time', // Set x-axis to time series
                    time: {
                        unit: 'second',
                        displayFormats: {
                            second: 'HH:mm:ss'
                        }
                    },
                    ticks: { color: '#94a3b8', source: 'auto', maxRotation: 0, autoSkip: true }, 
                    grid: { color: 'rgba(148, 163, 184, 0.1)'} 
                }, 
                y: { 
                    ticks: { color: '#94a3b8' }, 
                    grid: { color: 'rgba(148, 163, 184, 0.1)'} 
                } 
            },
            animation: false, // Disable all animations for performance
            plugins: { 
                legend: { 
                    labels: { color: '#e2e8f0' } 
                },
                downsample: {
                    enabled: true,
                    threshold: 200, // Start downsampling when we have > 200 points
                },
                tooltip: {
                    mode: 'index',
                    intersect: false,
                },
                zoom: {
                    pan: {
                        enabled: true,
                        mode: 'xy',
                    },
                    zoom: {
                        wheel: { enabled: true },
                        pinch: { enabled: true },
                        mode: 'xy',
                    }
                }
            } 
        } 
    });
}

function populateDocs() {
    docTabContent.innerHTML = `
        <div class="doc-section">
            <h4>🚀 How It Works</h4>
            <p>This bot follows a simple but powerful loop:</p>
            <ol class="list-decimal list-inside space-y-2 text-sm">
                <li><strong>Get Price</strong> - Fetch current market price from the selected connector</li>
                <li><strong>Analyze</strong> - Apply the chosen trading strategy to the price data</li>
                <li><strong>Decide</strong> - Generate BUY, SELL, or HOLD signals based on analysis</li>
                <li><strong>Execute</strong> - Place orders (paper or real) based on signals</li>
                <li><strong>Repeat</strong> - Continue the loop at the specified interval</li>
            </ol>
        </div>
        
        <div class="doc-section">
            <h4>🔧 Adding Custom Strategies</h4>
            <p>You can write your own trading strategies in Go:</p>
            <ol class="list-decimal list-inside space-y-2 text-sm">
                <li>Go to the "User Mods" tab</li>
                <li>Write your Go function named <code>strategyUserMod</code></li>
                <li>Click "Validate" to check syntax</li>
                <li>Click "Apply Mod & Recompile" to inject your code</li>
                <li>Select "User Mod (Custom)" in strategy dropdown</li>
            </ol>
        </div>
        
        <div class="doc-section">
            <h4>📝 Strategy Template</h4>
            <p>Copy this template to get started with custom strategies:</p>
            <pre><code>// Example: Buy when price increases, sell when it decreases
func strategyUserMod(bs *BotState) Signal {
    if len(bs.prices) &lt; 2 {
        return HOLD // Need at least 2 price points
    }
    
    currentPrice := bs.prices[len(bs.prices)-1]
    previousPrice := bs.prices[len(bs.prices)-2]
    
    // Simple momentum strategy
    if currentPrice &gt; previousPrice * 1.01 {
        return BUY
    }
    if currentPrice &lt; previousPrice * 0.99 {
        return SELL
    }
    return HOLD
}</code></pre>
        </div>
    `;
}

async function loadWasm(url) {
    goLog('info', `Loading WebAssembly module from ${url}...`);
    if (!WebAssembly.instantiateStreaming) { // Polyfill for browsers that don't support it
        WebAssembly.instantiateStreaming = async (resp, importObject) => {
            const source = await (await resp).arrayBuffer();
            return await WebAssembly.instantiate(source, importObject);
        };
    }
    const go = new Go();
    try {
        const result = await WebAssembly.instantiateStreaming(fetch(url), go.importObject);
        go.run(result.instance);
        goLog('success', 'WebAssembly module loaded and running.');
    } catch (err) {
        console.error('WASM instantiation failed:', err);
        goLog('error', `Failed to load WebAssembly module: ${err}.`);
        throw err; // Re-throw to be handled by the caller
    }
}

function updateModStatus(level, message) {
    modStatusDiv.innerHTML = '';
    modStatusDiv.className = `alert alert-${level} text-sm`;
    modStatusDiv.textContent = message;
    modStatusDiv.style.display = 'block';
}

function updateChartTheme() {
    if (!priceChart) return;
    const bodyStyles = getComputedStyle(document.body);
    const textColor = bodyStyles.getPropertyValue('--text-secondary').trim();
    const gridColor = bodyStyles.getPropertyValue('--chart-grid-color').trim();
    const primaryTextColor = bodyStyles.getPropertyValue('--text-primary').trim();

    priceChart.options.scales.x.ticks.color = textColor;
    priceChart.options.scales.y.ticks.color = textColor;
    priceChart.options.scales.x.grid.color = gridColor;
    priceChart.options.scales.y.grid.color = gridColor;
    priceChart.options.plugins.legend.labels.color = primaryTextColor;

    priceChart.update('none');
}

// CORRECTED: Moved event listener to the correct location in the script
document.addEventListener('DOMContentLoaded', async () => {
    try {
        await loadWasm("main.wasm");
    } catch (err) {
        goLog('error', `Initial WASM load failed: ${err}. Ensure server is running.`);
    }

    // --- Theme Setup ---
    function applyTheme(theme, isInitial = false) {
        document.body.classList.toggle('light-theme', theme === 'light');
        themeToggle.checked = (theme === 'dark');
        localStorage.setItem('theme', theme);
        if (!isInitial && priceChart) {
            updateChartTheme();
        }
    }
    themeToggle.addEventListener('change', () => {
        applyTheme(themeToggle.checked ? 'dark' : 'light');
    });
    const savedTheme = localStorage.getItem('theme') || (window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light');
    applyTheme(savedTheme, true);
    // --- End Theme Setup ---

    initializeTabs();
    initializeChart();
    populateDocs();
    updateChartTheme(); // Set initial chart theme after it's created

    try {
        const response = await fetch('main.go');
        if (!response.ok) {
            throw new Error(`Network response was not ok: ${response.statusText}`);
        }
        originalGoCode = await response.text();
        goLog('info', 'Go source code loaded for modding feature.');
    } catch (error) {
        console.error('Failed to load Go source:', error);
        goLog('error', 'Could not load Go source file. Modding will be disabled.');
        if (modCodeTextarea) modCodeTextarea.placeholder = 'Error: Could not load base Go source file for modding.';
        if (applyModButton) applyModButton.disabled = true;
    }
    
    // Initial UI setup
    createParamUI(connectorParamsDiv, connectorDefinitions, connectorSelect.value);
    createParamUI(strategyParamsDiv, strategyDefinitions, strategySelect.value);
    updateStrategyDescription();
    
    // Add other event listeners
    connectorSelect.addEventListener('change', () => createParamUI(connectorParamsDiv, connectorDefinitions, connectorSelect.value));
    strategySelect.addEventListener('change', () => {
        createParamUI(strategyParamsDiv, strategyDefinitions, strategySelect.value);
        updateStrategyDescription();
    });

    addSymbolBtn.addEventListener('click', () => {
        const newSymbol = newSymbolInput.value.trim().toUpperCase();
        if (!newSymbol) {
            goLog('warning', 'Symbol input is empty.');
            return;
        }
    
        if (Array.from(symbolSelect.options).some(opt => opt.value === newSymbol)) {
            goLog('info', `Symbol ${newSymbol} already exists.`);
            symbolSelect.value = newSymbol;
        } else {
            const newOption = new Option(newSymbol, newSymbol);
            symbolSelect.add(newOption);
            symbolSelect.value = newSymbol;
            goLog('success', `Added new symbol: ${newSymbol}`);
        }
        newSymbolInput.value = '';
    });

    saveConfigBtn.addEventListener('click', () => {
        try {
            const configString = generateFullConfig();
            const blob = new Blob([configString], { type: 'application/json' });
            const url = URL.createObjectURL(blob);
            const a = document.createElement('a');
            a.href = url;
            a.download = `ganymede-config-${Date.now()}.json`;
            document.body.appendChild(a);
            a.click();
            document.body.removeChild(a);
            URL.revokeObjectURL(url);
            goLog('info', 'Configuration saved successfully.');
        } catch (error) {
            goLog('error', `Failed to save configuration: ${error.message}`);
            console.error('Save config error:', error);
        }
    });

    loadConfigInput.addEventListener('change', (event) => {
        const file = event.target.files[0];
        if (!file) return;

        const reader = new FileReader();
        reader.onload = (e) => {
            try {
                const config = JSON.parse(e.target.result);
                applyConfig(config);
                goLog('success', 'Configuration loaded successfully.');
            } catch (error) {
                goLog('error', `Failed to load or parse configuration file: ${error.message}`);
                console.error('Load config error:', error);
            } finally {
                event.target.value = ''; // Reset input to allow loading same file again
            }
        };
        reader.onerror = () => {
            goLog('error', `Error reading file: ${reader.error}`);
            event.target.value = '';
        };
        reader.readAsText(file);
    });

    resetZoomBtn.addEventListener('click', () => {
        if (priceChart) {
            priceChart.resetZoom();
        }
    });

    tickIntervalInput.addEventListener('input', (e) => {
        tickValueSpan.textContent = `${e.target.value}s`;
    });

    clearLogsBtn.addEventListener('click', () => {
        logOutput.innerHTML = '';
        logs = [];
        goLog('info', 'Logs cleared.');
    });

    exportLogsBtn.addEventListener('click', () => {
        const blob = new Blob([JSON.stringify(logs, null, 2)], { type: 'application/json' });
        const url = URL.createObjectURL(blob);
        const a = document.createElement('a');
        a.href = url;
        a.download = `bot_logs_${new Date().toISOString()}.json`;
        a.click();
        URL.revokeObjectURL(url);
        goLog('info', 'Logs exported.');
    });

    startButton.addEventListener('click', () => {
        if (!validateConfig()) return;
        
        const config = generateFullConfig();
        console.log("Starting bot with config:", config);
        goLog('info', 'Attempting to start bot...');
        if (window.startBot) {
            window.startBot(config);
            startButton.disabled = true;
            stopButton.disabled = false;
        } else {
            goLog('error', 'WASM module not ready. Please wait.');
        }
    });

    validateModButton.addEventListener('click', async () => {
        const userCode = modCodeTextarea.value;
        if (!userCode.trim()) {
            updateModStatus('warning', 'Code is empty. Nothing to validate.');
            return;
        }
    
        updateModStatus('info', 'Validating your strategy...');
        applyModButton.disabled = true;
        validateModButton.disabled = true;
    
        try {
            const response = await fetch('/validate', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ code: userCode })
            });
    
            const result = await response.json();
            if (!response.ok || !result.success) {
                throw new Error(result.error || 'Unknown validation error');
            }
    
            updateModStatus('success', 'Validation successful! Your code looks good.');
    
        } catch (err) {
            updateModStatus('error', 'Validation Failed. See compiler output below:');
            const errorDetail = document.createElement('pre');
            errorDetail.className = 'mt-2 p-3 bg-slate-900/80 rounded-lg text-xs whitespace-pre-wrap';
            errorDetail.textContent = err.message;
            modStatusDiv.appendChild(errorDetail);
        } finally {
            applyModButton.disabled = false;
            validateModButton.disabled = false;
        }
    });

    applyModButton.addEventListener('click', async () => {
        const userCode = modCodeTextarea.value;
        if (!userCode.trim()) {
            updateModStatus('warning', 'Code is empty.');
            return;
        }

        updateModStatus('info', 'Compiling your strategy... This may take a moment.');
        applyModButton.disabled = true;
        validateModButton.disabled = true;

        try {
            const response = await fetch('/compile', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ code: userCode })
            });

            const result = await response.json();
            if (!response.ok || !result.success) {
                throw new Error(result.error || 'Unknown compilation error');
            }

            updateModStatus('success', `Compilation successful! New module at ${result.url}. Loading...`);
            
            if (window.stopBot && !stopButton.disabled) {
                goLog('info', 'Stopping current bot to load new module...');
                window.stopBot();
            }

            await loadWasm(result.url);

            strategySelect.querySelector('option[value="user_mod"]').hidden = false;
            strategySelect.value = 'user_mod';
            strategySelect.dispatchEvent(new Event('change'));
            goLog('success', 'Custom strategy loaded. Select "User Mod (Custom)" and start the bot.');
        } catch (err) {
            updateModStatus('error', 'Compilation Failed. See compiler output below:');
            const errorDetail = document.createElement('pre');
            errorDetail.className = 'mt-2 p-3 bg-slate-900/80 rounded-lg text-xs whitespace-pre-wrap';
            errorDetail.textContent = err.message;
            modStatusDiv.appendChild(errorDetail);
            goLog('error', 'Custom strategy compilation failed.');
        } finally {
            applyModButton.disabled = false;
            validateModButton.disabled = false;
        }
    });

    stopButton.addEventListener('click', () => {
        goLog('info', 'Attempting to stop bot...');
        if (window.stopBot) {
            window.stopBot();
            startButton.disabled = false;
            stopButton.disabled = true;
        } else {
            goLog('error', 'WASM module not ready.');
        }
    });
});