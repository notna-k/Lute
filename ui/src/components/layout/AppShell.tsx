import { type ReactNode } from 'react';
import { Navbar } from './Navbar';

interface AppShellProps {
  children: ReactNode;
}

export function AppShell({ children }: AppShellProps) {
  return (
    <div className='flex min-h-screen flex-col bg-bg'>
      <Navbar />
      <main className='flex-1'>
        <div className='mx-auto w-full max-w-7xl px-4 py-6 sm:px-6 sm:py-8'>
          {children}
        </div>
      </main>
    </div>
  );
}
