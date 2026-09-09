import { PortalAPI, PortalError, type Surface } from './portal.ts';

// Use the authenticated surface, not the destination URL. Logout revokes the
// current session and clears the unified cookie through the existing backend.
export async function endPortalSession(surface: Surface) {
  try {
    await new PortalAPI(surface).request('/logout', 'POST', {});
  } catch (error) {
    // An expired session is already signed out. Other failures must stay visible.
    if (!(error instanceof PortalError && error.status === 401)) throw error;
  }
}

export async function logoutPortal(
  surface: Surface,
  navigation: { leaveDeveloper: () => void; showLogin: () => void },
) {
  await endPortalSession(surface);
  if (surface === 'developer') {
    // Do not render the signed-out developer view: it starts merchant SSO
    // automatically and would race the navigation back to documentation.
    navigation.leaveDeveloper();
    return;
  }
  navigation.showLogin();
}
