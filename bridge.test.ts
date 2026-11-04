import assert from 'node:assert/strict';
import { test } from 'node:test';
import { createRelay } from './bridge.ts';
import type { RequestOptions } from './bridge.ts';

test('keeps credentials on the fixed origin, rereads refreshed tokens, and pins account', async () => {
    let token = 'first';
    let account = 'account-a';
    const requests: { url: string; options: RequestOptions }[] = [];
    const relay = createRelay(
        async () => ({ access_token: token, account_id: account }),
        async (url, options) => {
            requests.push({ url, options });
            const message = JSON.parse(options.body);
            return Response.json({ jsonrpc: '2.0', id: message.id, result: { protocolVersion: '2025-03-26' } }, {
                headers: { 'mcp-session-id': 'test-session' }
            });
        }
    );

    await relay({ jsonrpc: '2.0', id: 1, method: 'initialize', params: { capabilities: { sampling: {} } } });
    token = 'refreshed';
    await relay({ jsonrpc: '2.0', id: 2, method: 'tools/list' });
    assert.equal(requests[0].url, 'https://chatgpt.com/backend-api/ps/mcp');
    assert.equal(requests[0].options.redirect, 'error');
    assert.deepEqual(JSON.parse(requests[0].options.body).params.capabilities, {});
    assert.equal(requests[1].options.headers.Authorization, 'Bearer refreshed');
    assert.equal(requests[1].options.headers['X-OpenAI-Product-Sku'], 'codex');
    assert.equal(requests[1].options.headers['Mcp-Session-Id'], 'test-session');
    assert.equal(requests[1].options.headers['MCP-Protocol-Version'], '2025-03-26');

    account = 'account-b';
    await assert.rejects(relay({ jsonrpc: '2.0', id: 3, method: 'tools/call' }), /account changed/);
    assert.equal(requests.length, 2);
});

test('accepts chunked SSE and ignores unrelated notifications', async () => {
    const text = 'data: {"jsonrpc":"2.0","method":"notifications/progress"}\r\n\r\n'
        + 'data: {"jsonrpc":"2.0",\r\ndata: "id":8,"result":{"tools":[]}}\r\n\r\n';
    const body = new ReadableStream({
        start(controller) {
            for (const char of text) controller.enqueue(new TextEncoder().encode(char));
            controller.close();
        }
    });
    const relay = createRelay(
        async () => ({ access_token: 'test', account_id: 'a' }),
        async () => new Response(body, { headers: { 'Content-Type': 'text/event-stream' } })
    );
    assert.deepEqual(await relay({ jsonrpc: '2.0', id: 8, method: 'tools/list' }), {
        jsonrpc: '2.0', id: 8, result: { tools: [] }
    });
});

test('does not retry failed writes or include response bodies in errors', async () => {
    let calls = 0;
    const relay = createRelay(
        async () => ({ access_token: 'secret', account_id: 'a' }),
        async () => {
            calls++;
            return new Response('private upstream response', { status: 403 });
        }
    );
    await assert.rejects(relay({ jsonrpc: '2.0', id: 1, method: 'tools/call' }), error => {
        assert.ok(error instanceof Error);
        assert.match(error.message, /HTTP 403/);
        assert.doesNotMatch(error.message, /private|secret/);
        return true;
    });
    assert.equal(calls, 1);
});

test('rejects malformed upstream response envelopes', async () => {
    const relay = createRelay(
        async () => ({ access_token: 'test', account_id: 'a' }),
        async () => Response.json({ jsonrpc: '2.0', id: 1, result: 'invalid' })
    );
    await assert.rejects(relay({ jsonrpc: '2.0', id: 1, method: 'tools/list' }), /Invalid MCP response/);
});
