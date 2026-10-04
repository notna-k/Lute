// Plain prompts on stderr: a question, a hidden key, and a [y/N] confirmation. Callers
// check interactive() first; nothing here runs without a terminal.

import { createInterface } from 'node:readline/promises';

import { color, Exit } from './output.ts';

const label = (q: string) => color('dim', `? ${q}`, process.stderr);

export async function ask(question: string, hint = ''): Promise<string> {
  const rl = createInterface({ input: process.stdin, output: process.stderr });
  rl.on('SIGINT', () => interrupted());
  try {
    return (await rl.question(`${label(question)}${hint ? color('dim', ` ${hint}`, process.stderr) : ''}  `)).trim();
  } finally {
    rl.close();
  }
}

/** confirm accepts only y or Y; Enter and anything else mean No. */
export async function confirm(question: string): Promise<boolean> {
  const answer = await ask(color('yellow', `${question} [y/N]`, process.stderr));
  return answer === 'y' || answer === 'Y';
}

/** askHidden reads a secret with a bullet per character, so it never shows on screen. */
export function askHidden(question: string): Promise<string> {
  const stdin = process.stdin;
  process.stderr.write(`${label(question)}   `);
  return new Promise((resolve) => {
    let value = '';
    stdin.setRawMode(true);
    stdin.resume();
    stdin.setEncoding('utf8');
    const done = () => {
      stdin.setRawMode(false);
      stdin.pause();
      stdin.off('data', onData);
      process.stderr.write('\n');
    };
    const onData = (chunk: string) => {
      for (const ch of chunk) {
        if (ch === '\r' || ch === '\n') {
          done();
          resolve(value.trim());
          return;
        }
        if (ch === '\u0003') {
          done();
          interrupted();
        }
        if (ch === '\u007f' || ch === '\b') {
          if (value.length > 0) {
            value = value.slice(0, -1);
            process.stderr.write('\b \b');
          }
          continue;
        }
        if (ch >= ' ') {
          value += ch;
          process.stderr.write('•');
        }
      }
    };
    stdin.on('data', onData);
  });
}

// Ctrl-C arrives in an event handler, where a throw would not reach the command.
function interrupted(): never {
  process.stderr.write('\n');
  process.exit(Exit.interrupted);
}
