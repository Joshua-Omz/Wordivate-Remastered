import 'package:flutter/material.dart';

void main() {
  runApp(const WordivateApp());
}

class WordivateApp extends StatelessWidget {
  const WordivateApp({super.key});

  @override
  Widget build(BuildContext context) {
    return MaterialApp(
      title: 'Wordivate',
      theme: ThemeData(
        colorScheme: ColorScheme.fromSeed(seedColor: Colors.indigo),
        useMaterial3: true,
      ),
      home: const Scaffold(
        body: Center(
          child: Text('Wordivate'),
        ),
      ),
    );
  }
}
