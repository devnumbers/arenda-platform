import { useState } from 'react';
import { useLogin, useNotify } from 'react-admin';
import {
  Alert,
  Box,
  Button,
  Container,
  Paper,
  TextField,
  Typography,
} from '@mui/material';
import { isEmailValid } from './lib/email';
import { authProvider } from './authProvider';

export const LoginPage = () => {
  const login = useLogin();
  const notify = useNotify();

  const [step, setStep] = useState<'send' | 'verify'>('send');
  const [email, setEmail] = useState('');
  const [code, setCode] = useState('');
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const isEmailFormValid = isEmailValid(email.trim());

  // Обрезанный адрес либо inline-ошибка в форме: оба шага (отправка кода и
  // вход) валидируются одинаково, каноническую форму придаёт бэкенд.
  const validatedEmail = (): string | null => {
    const trimmedEmail = email.trim();
    if (!isEmailValid(trimmedEmail)) {
      setError('Введите корректный email');
      return null;
    }
    return trimmedEmail;
  };

  const handleSendCode = async (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const trimmedEmail = validatedEmail();
    if (trimmedEmail === null) {
      return;
    }

    setLoading(true);
    setError(null);

    try {
      await authProvider.sendLoginCode({ email: trimmedEmail });

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
    const trimmedEmail = validatedEmail();
    if (trimmedEmail === null) {
      return;
    }

    setLoading(true);
    setError(null);

    try {
      await login({ email: trimmedEmail, code }, '/');
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
            <Box component="form" onSubmit={(event) => void handleSendCode(event)} noValidate>
              <TextField
                margin="normal"
                required
                fullWidth
                label="Email"
                name="email"
                type="email"
                autoComplete="email"
                autoFocus
                value={email}
                onChange={(e) => setEmail(e.target.value)}
              />
              <Button
                type="submit"
                fullWidth
                variant="contained"
                sx={{ mt: 3, mb: 2 }}
                disabled={loading || !isEmailFormValid}
              >
                Отправить код
              </Button>
            </Box>
          ) : (
            <Box component="form" onSubmit={(event) => void handleLogin(event)} noValidate>
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
                Войти
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
