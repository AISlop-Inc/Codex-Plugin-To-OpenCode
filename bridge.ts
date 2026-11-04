#!/usr/bin/env node
import { realpathSync } from 'node:fs';
import { readFile } from 'node:fs/promises';
import { homedir } from 'node:os';
import { join } from 'node:path';
import { createInterface } from 'node:readline';
import { Readable } from 'node:stream';
import { pathToFileURL } from 'node:url';

const endpoint = 'https://chatgpt.com/backend-api/ps/mcp';

type CodexCredentials = { access_token: string; account_id: string };
type RequestId = string | number | null;
type McpRequest = {
    jsonrpc: '2.0';
    id?: RequestId;
    method: string;
    params?: Record<string, unknown>;
};
type McpResponse = {
    jsonrpc: '2.0';
    id: RequestId;
    result?: Record<string, unknown>;
    error?: { code: number; message: string };
};
export type RequestOptions = RequestInit & { headers: Record<string, string>; body: string };
type HttpRequest = (url: string, options: RequestOptions) => Promise<Response>;

async function readCodexAuth(): Promise<CodexCredentials> {
    try {
        const authPath = join(process.env.CODEX_HOME || join(homedir(), '.codex'), 'auth.json');
        const auth = JSON.parse(await readFile(authPath, 'utf8'));
        if (typeof auth.tokens?.access_token !== 'string' || !auth.tokens.access_token
            || typeof auth.tokens?.account_id !== 'string' || !auth.tokens.account_id) {
            throw new Error();
        }
        return auth.tokens;
    } catch {
        throw new Error('Codex ChatGPT credentials unavailable. Sign in through Codex.');
    }
}

function parseResponse(value: unknown): McpResponse {
    const message = value as Partial<McpResponse> | null;
    if (!message || message.jsonrpc !== '2.0'
        || (message.id !== null && typeof message.id !== 'string' && typeof message.id !== 'number')
        || (message.result !== undefined && (!message.result || typeof message.result !== 'object'))
        || (message.error !== undefined && (!message.error
            || typeof message.error.code !== 'number' || typeof message.error.message !== 'string'))) {
        throw new Error('Invalid MCP response.');
    }
    return message as McpResponse;
}

async function readResponse(response: Response, requestId: RequestId): Promise<McpResponse> {
    const contentType = response.headers.get('content-type') || '';
    if (contentType.includes('application/json')) {
        return parseResponse(await response.json());
    }
    if (!contentType.includes('text/event-stream') || !response.body) {
        throw new Error('Unexpected MCP response format.');
    }

    const stream = Readable.fromWeb(response.body);
    const lines = createInterface({ input: stream, crlfDelay: Infinity });
    let eventData: string[] = [];
    try {
        for await (const line of lines) {
            if (line.startsWith('data:')) {
                eventData.push(line.slice(5).replace(/^ /, ''));
            } else if (line === '' && eventData.length) {
                const message = JSON.parse(eventData.join('\n'));
                eventData = [];
                if (message.id === requestId) return parseResponse(message);
            }
        }
        throw new Error('MCP event stream ended without a response.');
    } finally {
        lines.close();
        stream.destroy();
    }
}

export function createRelay(
    loadAuth: () => Promise<CodexCredentials> = readCodexAuth,
    request: HttpRequest = fetch
) {
    let accountId: string | undefined;
    let sessionId: string | undefined;
    let protocolVersion: string | undefined;

    return async function relay(message: McpRequest): Promise<McpResponse | undefined> {
        const credentials = await loadAuth();
        if (accountId && credentials.account_id !== accountId) {
            throw new Error('Codex account changed. Reconnect this MCP server.');
        }
        accountId = credentials.account_id;

        const headers: Record<string, string> = {
            Authorization: `Bearer ${credentials.access_token}`,
            'ChatGPT-Account-ID': accountId,
            'X-OpenAI-Product-Sku': 'codex',
            'Content-Type': 'application/json',
            Accept: 'application/json, text/event-stream'
        };
        if (sessionId) headers['Mcp-Session-Id'] = sessionId;
        if (protocolVersion) headers['MCP-Protocol-Version'] = protocolVersion;

        const outgoing = message.method === 'initialize'
            ? { ...message, params: { ...message.params, capabilities: {} } }
            : message;
        const response = await request(endpoint, {
            method: 'POST',
            headers,
            body: JSON.stringify(outgoing),
            redirect: 'error',
            signal: AbortSignal.timeout(message.method === 'tools/call' ? 600000 : 60000)
        });

        if (!response.ok) {
            await response.body?.cancel();
            const hint = response.status === 401 || response.status === 403
                ? ' Refresh the ChatGPT login in Codex, then reconnect.'
                : '';
            throw new Error(`ChatGPT MCP returned HTTP ${response.status}.${hint}`);
        }
        sessionId = response.headers.get('mcp-session-id') || sessionId;
        if (message.id === undefined || response.status === 202 || response.status === 204) {
            await response.body?.cancel();
            if (message.id !== undefined) throw new Error('MCP request returned no result.');
            return;
        }

        const result = await readResponse(response, message.id);
        if (result.id !== message.id || result.jsonrpc !== '2.0') {
            throw new Error('MCP response does not match the request.');
        }
        if (message.method === 'initialize' && typeof result.result?.protocolVersion === 'string') {
            protocolVersion = result.result.protocolVersion;
        }
        return result;
    };
}

async function run() {
    const relay = createRelay();
    const input = createInterface({ input: process.stdin, crlfDelay: Infinity });
    const send = (message: McpResponse) => process.stdout.write(`${JSON.stringify(message)}\n`);

    for await (const line of input) {
        let message: McpRequest;
        try {
            message = JSON.parse(line);
            if (!message || message.jsonrpc !== '2.0' || typeof message.method !== 'string') {
                throw new Error();
            }
        } catch {
            send({ jsonrpc: '2.0', id: null, error: { code: -32600, message: 'Invalid MCP request.' } });
            continue;
        }

        // Never replay failed tool calls: they may already have changed remote data.
        relay(message).then(result => {
            if (result) send(result);
        }).catch((error: unknown) => {
            const detail = error instanceof TypeError || error instanceof SyntaxError
                ? 'MCP transport or response parsing failed.'
                : error instanceof Error
                    ? error.name === 'TimeoutError' ? 'MCP request timed out; outcome unknown.' : error.message
                    : 'MCP request failed.';
            if (message.id !== undefined) {
                send({ jsonrpc: '2.0', id: message.id, error: { code: -32000, message: detail } });
            } else {
                process.stderr.write(`${detail}\n`);
            }
        });
    }
}

if (process.argv[1] && import.meta.url === pathToFileURL(realpathSync(process.argv[1])).href) {
    const args = process.argv.slice(2);
    if (args.length === 1 && (args[0] === '--help' || args[0] === '-h')) {
        process.stdout.write(`Codex MCP Portal

Usage: codex-mcp-portal [--help]

Starts a stdio MCP server for existing ChatGPT connector tools.
Configure this command in any stdio-compatible MCP client.

Credentials: $CODEX_HOME/auth.json (default: ~/.codex/auth.json)
Token renewal is managed by Codex. No HTTP listener is started.
`);
    } else if (args.length) {
        process.stderr.write('Unknown arguments. Use codex-mcp-portal --help.\n');
        process.exitCode = 2;
    } else {
        await run();
    }
}
