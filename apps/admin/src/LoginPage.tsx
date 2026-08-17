import { useState } from 'react';
import { useLogin, useNotify } from 'react-admin';
import {
  Alert,
  Box,
  Button,
  CircularProgress,
  Container,
  Paper,
  TextField,
  Typography,
} from '@mui/material';
import { formatPhoneInput, isValidPhone, normalizePhone } from './phone';
import { authProvider } from './authProvider';

export const LoginPage = () => {
  const login = useLogin();
  const notify = useNotify();

  const [step, setStep] = useState<'send' | 'verify'>('send');
  const [phone, setPhone] = useState('');
  const [email, setEmail] = useState('');
  const [code, setCode] = useState('');
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const isPhoneValid = isValidPhone(phone);

  const handleSendCode = async (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const normalizedPhone = normalizePhone(phone);
    if (!isPhoneValid) {
      setError('Введите корректный номер телефона');
      return;
    }

    setLoading(true);
    setError(null);

    try {
      await authProvider.sendLoginCode({ phone: normalizedPhone, email });

      setStep('verify');
      notify('Код подтверждения отправлен на email', { type: 'success' });
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Не удалось отправить код');
    } finally {
      setLoading(false);
    }
  };

  const handleLogin = async (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const normalizedPhone = normalizePhone(phone);
    if (!isPhoneValid) {
      setError('Введите корректный номер телефона');
      return;
    }

    setLoading(true);
    setError(null);

    try {
      await login({ phone: normalizedPhone, email, code }, '/');
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Ошибка входа');
    } finally {
      setLoading(false);
    }
  };

  return (
    <Container component="main" maxWidth="xs">
      <Box
        sx={{
          marginTop: 8,
          display: 'flex',
          flexDirection: 'column',
          alignItems: 'center',
        }}
      >
        <Paper elevation={3} sx={{ padding: 4, width: '100%' }}>
          <Typography component="h1" variant="h5" align="center" gutterBottom>
            Рентли Admin
          </Typography>

          {error && (
            <Alert severity="error" sx={{ mb: 2 }}>
              {error}
            </Alert>
          )}

          {step === 'send' ? (
            <Box component="form" onSubmit={handleSendCode} noValidate>
              <TextField
                margin="normal"
                required
                fullWidth
                label="Телефон"
                name="phone"
                autoComplete="tel"
                autoFocus
                placeholder="+7 (000) 000-00-00"
                value={phone}
                onChange={(e) => setPhone(formatPhoneInput(e.target.value))}
              />
              <TextField
                margin="normal"
                required
                fullWidth
                label="Email"
                name="email"
                type="email"
                autoComplete="email"
                value={email}
                onChange={(e) => setEmail(e.target.value)}
              />
              <Button
                type="submit"
                fullWidth
                variant="contained"
                sx={{ mt: 3, mb: 2 }}
                disabled={loading || !isPhoneValid || !email}
              >
                {loading ? <CircularProgress size={24} /> : 'Отправить код'}
              </Button>
            </Box>
          ) : (
            <Box component="form" onSubmit={handleLogin} noValidate>
              <TextField
                margin="normal"
                required
                fullWidth
                label="Код подтверждения"
                name="code"
                autoFocus
                value={code}
                onChange={(e) => setCode(e.target.value)}
              />
              <Button
                type="submit"
                fullWidth
                variant="contained"
                sx={{ mt: 3, mb: 2 }}
                disabled={loading || !code}
              >
                {loading ? <CircularProgress size={24} /> : 'Войти'}
              </Button>
              <Button
                fullWidth
                variant="text"
                onClick={() => setStep('send')}
                disabled={loading}
              >
                Назад
              </Button>
            </Box>
          )}
        </Paper>
      </Box>
    </Container>
  );
};
