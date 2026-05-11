export type Language = 'curl' | 'python' | 'javascript' | 'typescript' | 'go';
export type WebhookResponseMode = 'sync' | 'async';

export function generateDeploymentCode(
  agentId: string,
  apiKey: string,
  language: Language,
  backendUrl: string = '',
): string {
  const url = `${backendUrl || window.location.origin}/api/agent/${agentId}/invoke`;

  switch (language) {
    case 'curl':
      return `curl -X POST "${url}" \\
  -H "X-API-Key: ${apiKey}" \\
  -H "Content-Type: application/json" \\
  -d '{"input": {"message": "Hello"}}'`;

    case 'python':
      return `import requests

response = requests.post(
    "${url}",
    headers={"X-API-Key": "${apiKey}"},
    json={"input": {"message": "Hello"}}
)
print(response.json())`;

    case 'javascript':
    case 'typescript':
      return `const response = await fetch("${url}", {
  method: "POST",
  headers: {
    "X-API-Key": "${apiKey}",
    "Content-Type": "application/json",
  },
  body: JSON.stringify({ input: { message: "Hello" } }),
});
const data = await response.json();
console.log(data);`;

    case 'go':
      return `// Go example
req, _ := http.NewRequest("POST", "${url}", strings.NewReader(\`{"input":{"message":"Hello"}}\`))
req.Header.Set("X-API-Key", "${apiKey}")
req.Header.Set("Content-Type", "application/json")
resp, _ := http.DefaultClient.Do(req)`;

    default:
      return '';
  }
}

export function generateWebhookCode(
  agentId: string,
  webhookUrl: string,
  _mode: WebhookResponseMode = 'async',
): string {
  return `# Webhook URL for agent ${agentId}
# POST ${webhookUrl}
curl -X POST "${webhookUrl}" \\
  -H "Content-Type: application/json" \\
  -d '{"data": {"message": "Hello from webhook"}}'`;
}
