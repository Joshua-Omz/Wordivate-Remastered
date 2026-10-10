import 'package:flutter/material.dart';

import 'feature_registry.dart';

class HomePage extends StatelessWidget {
  const HomePage({super.key});

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(title: const Text('Wordivate')),
      body: ListView.builder(
        itemCount: FeatureRegistry.sourceOfTruthFeatures.length,
        itemBuilder: (BuildContext context, int index) {
          return ListTile(
            title: Text(FeatureRegistry.sourceOfTruthFeatures[index]),
          );
        },
      ),
    );
  }
}
