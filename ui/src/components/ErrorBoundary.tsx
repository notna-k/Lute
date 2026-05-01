import { Component, type ErrorInfo, type ReactNode } from 'react';
import { Alert, Button } from '@/components/ui';

interface Props {
  children: ReactNode;
}

interface State {
  error: Error | null;
}

export class ErrorBoundary extends Component<Props, State> {
  state: State = { error: null };

  static getDerivedStateFromError(error: Error): State {
    return { error };
  }

  componentDidCatch(error: Error, info: ErrorInfo) {
    console.error('Unhandled error in UI:', error, info);
  }

  private reset = () => this.setState({ error: null });

  render() {
    if (this.state.error) {
      return (
        <div className='mx-auto max-w-2xl px-4 py-16'>
          <Alert tone='danger' title='Something went wrong'>
            <p className='mb-3'>
              {this.state.error.message || 'An unexpected error occurred.'}
            </p>
            <div className='flex gap-2'>
              <Button
                variant='secondary'
                onClick={() => window.location.reload()}
              >
                Reload page
              </Button>
              <Button variant='outline' onClick={this.reset}>
                Try again
              </Button>
            </div>
          </Alert>
        </div>
      );
    }
    return this.props.children;
  }
}
