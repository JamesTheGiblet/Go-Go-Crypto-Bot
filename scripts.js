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
        description: "Connect to Coinbase Pro exchange. Requires API credentials with trading permissions." 
    },
    "binance": { 
        name: "Binance", 
        params: { 
            "apiKey": { label: "API Key", type: "password" }, 
            "apiSecret": { label: "API Secret", type: "password" }
        }, 
        description: "Connect to Binance exchange. Requires API credentials with trading permissions." 
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
    const timestamp = new Date().toLocaleTimeString();
    priceChart.data.labels.push(timestamp);
    priceChart.data.datasets[0].data.push(newPrice);
    priceChart.data.datasets[1].data.push(null);
    priceChart.data.datasets[2].data.push(null);
    
    if (priceChart.data.labels.length > 100) {
        priceChart.data.labels.shift();
        priceChart.data.datasets.forEach(d => d.data.shift());
    }
    priceChart.update('none');
}

function goPlotSignal(signalType, price) {
    if (!priceChart) return;
    const dataLength = priceChart.data.labels.length;
    if (dataLength === 0) return;
    const datasetIndex = signalType === 'BUY' ? 1 : 2;
    priceChart.data.datasets[datasetIndex].data[dataLength - 1] = price;
    priceChart.update('none');
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

function validateConfig() { 
    const symbol = symbolSelect.value;
    const tickInterval = parseInt(tickIntervalInput.value, 10);
    
    if (!symbol) {
        alert('Please select a trading pair.');
        return false;
    }
    
    if (tickInterval < 1 || tickInterval > 60) {
        alert('Tick interval must be between 1 and 60 seconds.');
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
            labels: [], 
            datasets: [ 
                { 
                    label: 'Price (USD)', 
                    data: [], 
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
                } 
            ] 
        }, 
        options: { 
            responsive: true, 
            maintainAspectRatio: false, 
            scales: { 
                x: { 
                    ticks: { color: '#94a3b8' }, 
                    grid: { color: 'rgba(148, 163, 184, 0.1)'} 
                }, 
                y: { 
                    ticks: { color: '#94a3b8' }, 
                    grid: { color: 'rgba(148, 163, 184, 0.1)'} 
                } 
            }, 
            plugins: { 
                legend: { 
                    labels: { color: '#e2e8f0' } 
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

// CORRECTED: Moved event listener to the correct location in the script
document.addEventListener('DOMContentLoaded', async () => {
    // --- WASM Loading ---
    if (!WebAssembly.instantiateStreaming) { // Polyfill for browsers that don't support it
        WebAssembly.instantiateStreaming = async (resp, importObject) => {
            const source = await (await resp).arrayBuffer();
            return await WebAssembly.instantiate(source, importObject);
        };
    }

    const go = new Go();
    try {
        // Fetch and instantiate the WASM file
        const result = await WebAssembly.instantiateStreaming(fetch("main.wasm"), go.importObject);
        // Run the Go program. This is non-blocking and will run main() in a goroutine.
        go.run(result.instance);
        goLog('info', 'WebAssembly module loaded and running.');
    } catch (err) {
        console.error('WASM instantiation failed:', err);
        goLog('error', `Failed to load WebAssembly module: ${err}. Make sure you are running a local web server.`);
    }
    // --- End of WASM Loading ---

    initializeTabs();
    initializeChart();
    populateDocs();

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