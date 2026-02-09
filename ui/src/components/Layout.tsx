import { ReactNode } from 'react';
import { Link, useLocation, useNavigate } from 'react-router-dom';
import {
  AppBar,
  Toolbar,
  Typography,
  Button,
  Box,
  Avatar,
  Container,
  Tabs,
  Tab,
} from '@mui/material';
import { useAuth } from '../contexts/AuthContext';
import { logout } from '../services/authService';

interface LayoutProps {
  children: ReactNode;
}

const Layout = ({ children }: LayoutProps) => {
  const { user } = useAuth();
  const location = useLocation();
  const navigate = useNavigate();

  const handleLogout = async () => {
    try {
      await logout();
      navigate('/login');
    } catch (error) {
      console.error('Error logging out:', error);
    }
  };

  const getTabValue = (): number | false => {
    if (location.pathname === '/dashboard') return 0;
    if (location.pathname === '/machines') return 1;
    if (location.pathname === '/public-machines') return 2;
    return false;
  };

  const handleTabChange = (_event: React.SyntheticEvent, newValue: number) => {
    const paths = ['/dashboard', '/machines', '/public-machines'];
    navigate(paths[newValue]);
  };

  const tabValue = getTabValue();

  return (
    <Box sx={{ display: 'flex', flexDirection: 'column', minHeight: '100vh', bgcolor: 'grey.50' }}>
      <AppBar position="static" elevation={0} sx={{ borderBottom: 1, borderColor: 'divider' }}>
        <Toolbar>
          <Typography
            component={Link}
            to="/"
            variant="h5"
            sx={{
              fontWeight: 'bold',
              color: 'primary.main',
              textDecoration: 'none',
              mr: 4,
            }}
          >
            Lute
          </Typography>
          {user && tabValue !== false && (
            <Tabs
              value={tabValue}
              onChange={handleTabChange}
              sx={{ flexGrow: 1 }}
              textColor="inherit"
            >
              <Tab label="Dashboard" />
              <Tab label="My Machines" />
              <Tab label="Public Machines" />
            </Tabs>
          )}
          {user && (
            <Box sx={{ display: 'flex', alignItems: 'center', gap: 2 }}>
              <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                {user.photoURL && (
                  <Avatar src={user.photoURL} alt={user.displayName || 'User'} sx={{ width: 32, height: 32 }} />
                )}
                <Typography variant="body2" sx={{ display: { xs: 'none', sm: 'block' } }}>
                  {user.displayName || user.email}
                </Typography>
              </Box>
              <Button
                variant="contained"
                onClick={handleLogout}
                size="small"
              >
                Logout
              </Button>
            </Box>
          )}
        </Toolbar>
      </AppBar>
      <Container maxWidth="xl" sx={{ py: 3, flex: 1 }}>
        {children}
      </Container>
    </Box>
  );
};

export default Layout;
