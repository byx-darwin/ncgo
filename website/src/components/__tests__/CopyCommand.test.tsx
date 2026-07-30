import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';
import CopyCommand from '../CopyCommand';

describe('CopyCommand', () => {
  it('shows the command text', () => {
    render(<CopyCommand command="go install github.com/byx-darwin/ncgo@latest" />);
    expect(screen.getByText('go install github.com/byx-darwin/ncgo@latest')).toBeInTheDocument();
  });

  it('copies command to clipboard on click', async () => {
    const user = userEvent.setup();
    const cmd = 'go install github.com/byx-darwin/ncgo@latest';
    render(<CopyCommand command={cmd} />);
    await user.click(screen.getByRole('button', { name: 'copy' }));
    expect(navigator.clipboard.writeText).toHaveBeenCalledWith(cmd);
  });
});
